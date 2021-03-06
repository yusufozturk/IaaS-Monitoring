package services

import (
	client "github.com/influxdata/influxdb/client/v2"
	"kr/paasta/monitoring/openstack/models"
	"kr/paasta/monitoring/openstack/utils"
	"kr/paasta/monitoring/openstack/dao"
	"kr/paasta/monitoring/openstack/integration"
	"github.com/rackspace/gophercloud"
	"reflect"
	"sync"
	"encoding/json"
	"strconv"
	"strings"
)

type ComputeNodeService struct {
	openstackProvider models.OpenstackProvider
	provider          *gophercloud.ProviderClient
	influxClient 	 client.Client
}

func GetComputeNodeService(openstackProvider models.OpenstackProvider, provider *gophercloud.ProviderClient, influxClient client.Client) *ComputeNodeService {
	return &ComputeNodeService{
		openstackProvider: openstackProvider,
		provider:         provider,
		influxClient: 	influxClient,
	}
}



func (n ComputeNodeService) GetComputeNodeSummary(apiRequest models.NodeReq)([]models.NodeResources, models.ErrMessage){

	var result []models.NodeResources


	//Compute Node목록 및 Summary정보를 조회한다.
	computeInfoList, err := integration.GetNova(n.openstackProvider, n.provider).GetComputeNodeResources()

	errMsg := utils.GetError().GetCheckErrorMessage(err)
	if err != nil {
		return computeInfoList, errMsg
	}
	//Compute Node의 Status를 조회한다.
	computeResult := computeInfoList
	var errs []models.ErrMessage
	if(len(computeInfoList) > 0){

		for idx, computeNode :=range computeInfoList {

			var req models.NodeReq
			req.HostName = computeNode.Hostname
			cpuData, memData,  agentForwarderData, agentCollectorData,  runningVmsCnt , err := getNodeSummary_Sub(req, n.influxClient)

			if err != nil {
				errs = append(errs, err)
			}

			cpuUsage  := utils.GetDataFloatFromInterfaceSingle(cpuData)
			memUsage    := utils.GetDataFloatFromInterfaceSingle(memData)

			agentForwarderStatus   := utils.GetDataFloatFromInterfaceSingle(agentForwarderData)
			agentCollectorStatus   := utils.GetDataFloatFromInterfaceSingle(agentCollectorData)

			computeResult[idx].CpuUsage     = cpuUsage
			computeResult[idx].MemUsage     = 100 - memUsage
			computeResult[idx].RunningVms   = runningVmsCnt
			if agentForwarderStatus == 1 && agentCollectorStatus == 1{
				computeResult[idx].AgentStatus = "OK"
			}else{
				if agentForwarderStatus != 1{
					if agentForwarderStatus == 0{
						computeResult[idx].AgentStatus = "Forwarder Down"
					}else if agentForwarderStatus == -1{
						computeResult[idx].AgentStatus = "Forwarder UnKnown"
					}
				}else if agentCollectorStatus != 1{
					if agentCollectorStatus == 0{
						computeResult[idx].AgentStatus = "Collector Down"
					}else if agentForwarderStatus == -1{
						computeResult[idx].AgentStatus = "Collector UnKnown"
					}
				}
			}
		}
	}
	//==========================================================================
	// Error가 여러건일 경우 대해 고려해야함.
	if len(errs) > 0 {
		var returnErrMessage string
		for _, err := range errs{
			returnErrMessage = returnErrMessage + " " + err["Message"].(string)
		}
		errMessage := models.ErrMessage{
			"Message": returnErrMessage ,
		}
		return nil, errMessage
	}
	//==========================================================================

	//조회 조건
	if apiRequest.HostName != "" {
		for _, compute := range computeInfoList{

			if strings.Contains(compute.Hostname, apiRequest.HostName){
				result = append(result, compute)
			}
		}
		return result, nil
	}else{
		return computeInfoList, nil
	}


}


//CPU 사용률
func (n ComputeNodeService) GetComputeNodeCpuUsageList(request models.DetailReq)(result []map[string]interface{}, _ models.ErrMessage){

	cpuUsageResp, err := dao.GetNodeDao(n.influxClient).GetNodeCpuUsageList(request)
	if err != nil {
		models.MonitLogger.Error(err)
		return result, err
	}else {
		cpuUsage, _ := utils.GetResponseConverter().InfluxConverterList(cpuUsageResp, models.METRIC_NAME_CPU_USAGE)

		datamap := cpuUsage[models.RESULT_DATA_NAME].([]map[string]interface{})
		for _, data := range datamap{

			swapFree := data["usage"].(json.Number)
			convertData, _ := strconv.ParseFloat(swapFree.String(),64)
			//swap 사용률을 구한다. ( 100 -  freeUsage)
			data["usage"] = utils.RoundFloatDigit2(convertData)
		}

		result = append(result,cpuUsage )
		return result, nil
	}
}

//CPU Load Avg_1m
func (n ComputeNodeService) GetComputeNodeCpuLoad1mList(request models.DetailReq)(result []map[string]interface{}, _ models.ErrMessage){

	cpuLoad1mResp, err := dao.GetNodeDao(n.influxClient).GetNodeCpuLoadList(request, "1m")
	cpuLoad5mResp, err := dao.GetNodeDao(n.influxClient).GetNodeCpuLoadList(request, "5m")
	cpuLoad15mResp, err := dao.GetNodeDao(n.influxClient).GetNodeCpuLoadList(request, "15m")

	if err != nil {
		models.MonitLogger.Error(err)
		return result, err
	}else {
		cpu1mLoad, _ := utils.GetResponseConverter().InfluxConverterList(cpuLoad1mResp, models.METRIC_NAME_CPU_LOAD_1M)
		cpu5mLoad, _ := utils.GetResponseConverter().InfluxConverterList(cpuLoad5mResp, models.METRIC_NAME_CPU_LOAD_5M)
		cpu15mLoad, _ := utils.GetResponseConverter().InfluxConverterList(cpuLoad15mResp, models.METRIC_NAME_CPU_LOAD_15M)

		datamap1m := cpu1mLoad[models.RESULT_DATA_NAME].([]map[string]interface{})
		for _, data := range datamap1m{

			swapFree := data["usage"].(json.Number)
			convertData, _ := strconv.ParseFloat(swapFree.String(),64)

			data["usage"] = utils.RoundFloatDigit2(convertData)
		}

		datamap5m := cpu5mLoad[models.RESULT_DATA_NAME].([]map[string]interface{})
		for _, data := range datamap5m{

			swapFree := data["usage"].(json.Number)
			convertData, _ := strconv.ParseFloat(swapFree.String(),64)

			data["usage"] = utils.RoundFloatDigit2(convertData)
		}

		datamap15m := cpu15mLoad[models.RESULT_DATA_NAME].([]map[string]interface{})
		for _, data := range datamap15m{

			swapFree := data["usage"].(json.Number)
			convertData, _ := strconv.ParseFloat(swapFree.String(),64)

			data["usage"] = utils.RoundFloatDigit2(convertData)
		}
		result = append(result, cpu1mLoad)
		result = append(result, cpu5mLoad)
		result = append(result, cpu15mLoad)

		return result, nil
	}
}


//Memory Swap Usage
func (n ComputeNodeService) GetComputeNodeMemoryUsageList(request models.DetailReq)(result []map[string]interface{}, _ models.ErrMessage){

	memoryResp, err := dao.GetNodeDao(n.influxClient).GetNodeMemoryUsageList(request)
	if err != nil {
		models.MonitLogger.Error(err)
		return result, err
	}else {
		memoryUsage, _ := utils.GetResponseConverter().InfluxConverterList(memoryResp, models.METRIC_NAME_MEMORY_USAGE)

		datamap := memoryUsage[models.RESULT_DATA_NAME].([]map[string]interface{})

		for _, data := range datamap{
			data["usage"] =  100 - utils.TypeChecker_float64(data["usage"]).(float64)
		}

		result = append(result, memoryUsage )
		return result, nil
	}
}

//Memory Swap Usage
func (n ComputeNodeService) GetComputeNodeSwapUsageList(request models.DetailReq)(result []map[string]interface{}, _ models.ErrMessage){

	cpuLoadResp, err := dao.GetNodeDao(n.influxClient).GetNodeSwapMemoryFreeUsageList(request)
	if err != nil {
		models.MonitLogger.Error(err)
		return result, err
	}else {
		swapFreeUsage, _ := utils.GetResponseConverter().InfluxConverterList(cpuLoadResp, models.METRIC_NAME_MEMORY_SWAP)

		datamap := swapFreeUsage[models.RESULT_DATA_NAME].([]map[string]interface{})
		for _, data := range datamap{

			swapFree := data["usage"].(json.Number)
			convertData, _ := strconv.ParseFloat(swapFree.String(),64)
			//swap 사용률을 구한다. ( 100 -  freeUsage)
			data["usage"] = utils.RoundFloatDigit2(100 - convertData)
		}

		result = append(result, swapFreeUsage)
		return result, nil
	}
}


func  getNodeSummary_Sub(request models.NodeReq, f client.Client) (map[string]interface{}, map[string]interface{}, map[string]interface{},
	map[string]interface{}, int, models.ErrMessage) {
	var cpuResp, memResp,  agentForwarderResp, agentCollectorResp, instanceListResp client.Response

	var errs []models.ErrMessage
	var err models.ErrMessage
	var wg sync.WaitGroup
	wg.Add(5)

	for i := 0; i < 5; i++ {
		go func(wg *sync.WaitGroup, index int) {

			switch index {
			case 0 :
				cpuResp, err = dao.GetNodeDao(f).GetNodeCpuUsage(request)
				if err != nil {
					errs = append(errs, err)
				}
			case 1 :
				memResp, err = dao.GetNodeDao(f).GetNodeMemoryUsage(request)
				if err != nil {
					errs = append(errs, err)
				}
			case 2 :
				agentForwarderResp, err = dao.GetNodeDao(f).GetAgentProcessStatus(request, "forwarder")
				if err != nil {
					errs = append(errs, err)
				}
			case 3 :
				agentCollectorResp, err = dao.GetNodeDao(f).GetAgentProcessStatus(request,"collector")
				if err != nil {
					errs = append(errs, err)
				}
			case 4 :
				instanceListResp, err = dao.GetNodeDao(f).GetAliveInstanceListByNodename(request, false)
				if err != nil {
					errs = append(errs, err)
				}
			default:
				break
			}
			wg.Done()
		}(&wg, i)
	}
	wg.Wait()

	//==========================================================================
	// Error가 여러건일 경우 대해 고려해야함.
	if len(errs) > 0 {
		var returnErrMessage string
		for _, err := range errs{
			returnErrMessage = returnErrMessage + " " + err["Message"].(string)
		}
		errMessage := models.ErrMessage{
			"Message": returnErrMessage ,
		}
		return nil, nil, nil,  nil, 0, errMessage
	}
	//==========================================================================
	cpuUsage, _   := utils.GetResponseConverter().InfluxConverter(cpuResp)
	memUsage, _   := utils.GetResponseConverter().InfluxConverter(memResp)
	agentForwarder, _ := utils.GetResponseConverter().InfluxConverter(agentForwarderResp)
	agentCollector, _ := utils.GetResponseConverter().InfluxConverter(agentCollectorResp)
	instanceList, _   := utils.GetResponseConverter().InfluxConverterToMap(instanceListResp)
	var instanceGuidList []string
	//valueList, _ := utils.GetResponseConverter().InfluxConverterToMap(instanceList)
	for _, value := range instanceList{

		instanceGuid := reflect.ValueOf(value["resource_id"]).String()

		if utils.StringArrayDistinct(instanceGuid, instanceGuidList) == false{
			instanceGuidList = append(instanceGuidList, instanceGuid)
		}
	}

	return cpuUsage, memUsage, agentForwarder, agentCollector, len(instanceGuidList), nil
}


//Memory Swap Usage
func (n ComputeNodeService) GetNodeDiskUsageList(request models.DetailReq)(result []map[string]interface{}, _ models.ErrMessage){

	mountPointResp, err := dao.GetNodeDao(n.influxClient).GetMountPointList(request)
	if err != nil {
		models.MonitLogger.Error(err)
		return result, err
	}else {
		mountPointList,_ := utils.GetResponseConverter().GetMountPointList(mountPointResp)


		var mountPointSelectList []string
		for idx := range  mountPointList{
			//Boot Mount Point는 제외
			if strings.Contains(mountPointList[idx], "/boot") == false{
				mountPointSelectList = append( mountPointSelectList, mountPointList[idx])
			}

		}

		for _, value := range mountPointSelectList{
			request.MountPoint = value

			diskResp , _ := dao.GetNodeDao(n.influxClient).GetNodeDiskUsage(request)
			diskUsage, _ := utils.GetResponseConverter().InfluxConverterList(diskResp, value)

			datamap := diskUsage[models.RESULT_DATA_NAME].([]map[string]interface{})
			for _, data := range datamap{

				swapFree := data["usage"].(json.Number)
				convertData, _ := strconv.ParseFloat(swapFree.String(),64)
				//swap 사용률을 구한다. ( 100 -  freeUsage)
				data["usage"] = utils.RoundFloatDigit2(convertData)
			}

			result = append(result, diskUsage)
		}


		//result = mountPointList

		return result, nil
	}
}



//Disk IO Read Byte
func (n ComputeNodeService) GetNodeDiskIoReadList(request models.DetailReq)(result []map[string]interface{}, _ models.ErrMessage){

	mountPointResp, err := dao.GetNodeDao(n.influxClient).GetMountPointList(request)
	if err != nil {
		models.MonitLogger.Error(err)
		return result, err
	}else {
		mountPointList,_ := utils.GetResponseConverter().GetMountPointList(mountPointResp)

		var mountPointSelectList []string
		for idx := range  mountPointList{
			//Boot Mount Point는 제외
			if strings.Contains(mountPointList[idx], "/boot") == false{
				mountPointSelectList = append( mountPointSelectList, mountPointList[idx])
			}

		}

		for _, value := range mountPointSelectList{
			request.MountPoint = value

			diskResp , _ := dao.GetNodeDao(n.influxClient).GetNodeDiskIoReadKbyte(request)
			diskUsage, _ := utils.GetResponseConverter().InfluxConverterList(diskResp, value)

			datamap := diskUsage[models.RESULT_DATA_NAME].([]map[string]interface{})
			for _, data := range datamap{

				swapFree := data["usage"].(json.Number)
				convertData, _ := strconv.ParseFloat(swapFree.String(),64)
				//swap 사용률을 구한다. ( 100 -  freeUsage)
				data["usage"] = utils.RoundFloatDigit2(convertData)
			}

			result = append(result, diskUsage)
		}


		//result = mountPointList

		return result, nil
	}
}

//Disk IO Write Byte
func (n ComputeNodeService) GetNodeDiskIoWriteList(request models.DetailReq)(result []map[string]interface{}, _ models.ErrMessage){

	mountPointResp, err := dao.GetNodeDao(n.influxClient).GetMountPointList(request)
	if err != nil {
		models.MonitLogger.Error(err)
		return result, err
	}else {
		mountPointList,_ := utils.GetResponseConverter().GetMountPointList(mountPointResp)

		var mountPointSelectList []string
		for idx := range  mountPointList{
			//Boot Mount Point는 제외
			if strings.Contains(mountPointList[idx], "/boot") == false{
				mountPointSelectList = append( mountPointSelectList, mountPointList[idx])
			}

		}

		for _, value := range mountPointSelectList{
			request.MountPoint = value

			diskResp , _ := dao.GetNodeDao(n.influxClient).GetNodeDiskIoWriteKbyte(request)
			diskUsage, _ := utils.GetResponseConverter().InfluxConverterList(diskResp, value)

			datamap := diskUsage[models.RESULT_DATA_NAME].([]map[string]interface{})
			for _, data := range datamap{

				swapFree := data["usage"].(json.Number)
				convertData, _ := strconv.ParseFloat(swapFree.String(),64)
				//swap 사용률을 구한다. ( 100 -  freeUsage)
				data["usage"] = utils.RoundFloatDigit2(convertData)
			}

			result = append(result, diskUsage)
		}

		return result, nil
	}
}


//Disk IO Write Byte
func (n ComputeNodeService) GetNodeNetworkInOutKByteList(request models.DetailReq)(result []map[string]interface{}, _ models.ErrMessage){

	networkInEthResp, err := dao.GetNodeDao(n.influxClient).GetNodeNetworkKbyte(request, "in", "em")
	networkInVxResp, err := dao.GetNodeDao(n.influxClient).GetNodeNetworkKbyte(request, "in", "vxlan")

	networkEthOutResp, err := dao.GetNodeDao(n.influxClient).GetNodeNetworkKbyte(request, "out", "em")
	networkVxOutResp, err := dao.GetNodeDao(n.influxClient).GetNodeNetworkKbyte(request, "out", "vxlan")

	if err != nil {
		models.MonitLogger.Error(err)
		return result, err
	}else {
		networkEthInUsage, _ := utils.GetResponseConverter().InfluxConverterList(networkInEthResp, models.METRIC_NAME_NETWORK_ETH_IN)
		networkVxInUsage, _ := utils.GetResponseConverter().InfluxConverterList(networkInVxResp, models.METRIC_NAME_NETWORK_VX_IN)

		networkEthOutUsage, _ := utils.GetResponseConverter().InfluxConverterList(networkEthOutResp, models.METRIC_NAME_NETWORK_ETH_OUT)
		networkVxOutUsage, _ := utils.GetResponseConverter().InfluxConverterList(networkVxOutResp, models.METRIC_NAME_NETWORK_VX_OUT)

		inEthDatamap := networkEthInUsage[models.RESULT_DATA_NAME].([]map[string]interface{})
		for _, data := range inEthDatamap{

			inByte := data["usage"].(json.Number)
			convertData, _ := strconv.ParseFloat(inByte.String(),64)
			data["usage"] = utils.RoundFloatDigit2(convertData)
		}

		inVxDatamap := networkVxInUsage[models.RESULT_DATA_NAME].([]map[string]interface{})
		for _, data := range inVxDatamap{

			inByte := data["usage"].(json.Number)
			convertData, _ := strconv.ParseFloat(inByte.String(),64)
			data["usage"] = utils.RoundFloatDigit2(convertData)
		}

		outEthDatamap := networkEthOutUsage[models.RESULT_DATA_NAME].([]map[string]interface{})
		for _, data := range outEthDatamap{

			outByte := data["usage"].(json.Number)
			convertData, _ := strconv.ParseFloat(outByte.String(),64)

			data["usage"] = utils.RoundFloatDigit2(convertData)
		}

		outVxDatamap := networkVxOutUsage[models.RESULT_DATA_NAME].([]map[string]interface{})
		for _, data := range outVxDatamap{

			outByte := data["usage"].(json.Number)
			convertData, _ := strconv.ParseFloat(outByte.String(),64)

			data["usage"] = utils.RoundFloatDigit2(convertData)
		}

		result = append(result, networkEthInUsage)
		result = append(result, networkVxInUsage)
		result = append(result, networkEthOutUsage)
		result = append(result, networkVxOutUsage)


		return result, nil
	}
}


//Network In/Out Error
func (n ComputeNodeService) GetNodeNetworkInOutErrorList(request models.DetailReq)(result []map[string]interface{}, _ models.ErrMessage){

	networkInEthResp, err := dao.GetNodeDao(n.influxClient).GetNodeNetworkError(request, "in",  "em")
	networkInVxResp, err := dao.GetNodeDao(n.influxClient).GetNodeNetworkError(request, "in",  "vxlan")
	networkOuEthtResp, err := dao.GetNodeDao(n.influxClient).GetNodeNetworkError(request, "out", "em")
	networkOutVxResp, err := dao.GetNodeDao(n.influxClient).GetNodeNetworkError(request, "out", "vxlan")
	if err != nil {
		models.MonitLogger.Error(err)
		return result, err
	}else {
		networkInEthError, _ := utils.GetResponseConverter().InfluxConverterList(networkInEthResp, models.METRIC_NAME_NETWORK_ETH_IN_ERROR)
		networkInVxError, _ := utils.GetResponseConverter().InfluxConverterList(networkInVxResp, models.METRIC_NAME_NETWORK_VX_IN_ERROR)

		networkOutEthError, _ := utils.GetResponseConverter().InfluxConverterList(networkOuEthtResp, models.METRIC_NAME_NETWORK_ETH_OUT_ERROR)
		networkOutVxError, _ := utils.GetResponseConverter().InfluxConverterList(networkOutVxResp, models.METRIC_NAME_NETWORK_VX_OUT_ERROR)

		inEthDatamap := networkInEthError[models.RESULT_DATA_NAME].([]map[string]interface{})
		for _, data := range inEthDatamap{

			inByte := data["usage"].(json.Number)
			convertData, _ := strconv.ParseFloat(inByte.String(),64)
			data["usage"] = utils.RoundFloatDigit2(convertData)
		}

		inVxDatamap := networkInVxError[models.RESULT_DATA_NAME].([]map[string]interface{})
		for _, data := range inVxDatamap{

			inByte := data["usage"].(json.Number)
			convertData, _ := strconv.ParseFloat(inByte.String(),64)
			data["usage"] = utils.RoundFloatDigit2(convertData)
		}

		outEthDatamap := networkOutEthError[models.RESULT_DATA_NAME].([]map[string]interface{})
		for _, data := range outEthDatamap{

			outByte := data["usage"].(json.Number)
			convertData, _ := strconv.ParseFloat(outByte.String(),64)

			data["usage"] = utils.RoundFloatDigit2(convertData)
		}

		outVxDatamap := networkOutVxError[models.RESULT_DATA_NAME].([]map[string]interface{})
		for _, data := range outVxDatamap{

			outByte := data["usage"].(json.Number)
			convertData, _ := strconv.ParseFloat(outByte.String(),64)

			data["usage"] = utils.RoundFloatDigit2(convertData)
		}

		result = append(result, networkInEthError)
		result = append(result, networkInVxError)
		result = append(result, networkOutEthError)
		result = append(result, networkOutVxError)

		return result, nil
	}
}

//Network Dropped packets
func (n ComputeNodeService) GetNodeNetworkDropPacketList(request models.DetailReq)(result []map[string]interface{}, _ models.ErrMessage){

	networkInEthResp, err := dao.GetNodeDao(n.influxClient).GetNodeNetworkDropPacket(request, "in", "em")
	networkInVxResp, err := dao.GetNodeDao(n.influxClient).GetNodeNetworkDropPacket(request, "in", "vxlan")

	networkOutEthResp, err := dao.GetNodeDao(n.influxClient).GetNodeNetworkDropPacket(request, "out", "em")
	networkOutVxResp, err := dao.GetNodeDao(n.influxClient).GetNodeNetworkDropPacket(request, "out", "vxlan")

	if err != nil {
		models.MonitLogger.Error(err)
		return result, err
	}else {
		networkInEthError, _ := utils.GetResponseConverter().InfluxConverterList(networkInEthResp, models.METRIC_NAME_NETWORK_ETH_IN_DROPPED_PACKET)
		networkInVxError, _ := utils.GetResponseConverter().InfluxConverterList(networkInVxResp, models.METRIC_NAME_NETWORK_VX_IN_DROPPED_PACKET)

		networkOutEthError, _ := utils.GetResponseConverter().InfluxConverterList(networkOutEthResp, models.METRIC_NAME_NETWORK_ETH_OUT_DROPPED_PACKET)
		networkOutVxError, _ := utils.GetResponseConverter().InfluxConverterList(networkOutVxResp, models.METRIC_NAME_NETWORK_VX_OUT_DROPPED_PACKET)

		inEthDatamap := networkInEthError[models.RESULT_DATA_NAME].([]map[string]interface{})
		for _, data := range inEthDatamap{

			inByte := data["usage"].(json.Number)
			convertData, _ := strconv.ParseFloat(inByte.String(),64)
			data["usage"] = utils.RoundFloatDigit2(convertData)
		}

		inVxDatamap := networkInVxError[models.RESULT_DATA_NAME].([]map[string]interface{})
		for _, data := range inVxDatamap{

			inByte := data["usage"].(json.Number)
			convertData, _ := strconv.ParseFloat(inByte.String(),64)
			data["usage"] = utils.RoundFloatDigit2(convertData)
		}

		outEthDatamap := networkOutEthError[models.RESULT_DATA_NAME].([]map[string]interface{})
		for _, data := range outEthDatamap{

			outByte := data["usage"].(json.Number)
			convertData, _ := strconv.ParseFloat(outByte.String(),64)

			data["usage"] = utils.RoundFloatDigit2(convertData)
		}

		outVxDatamap := networkOutVxError[models.RESULT_DATA_NAME].([]map[string]interface{})
		for _, data := range outVxDatamap{

			outByte := data["usage"].(json.Number)
			convertData, _ := strconv.ParseFloat(outByte.String(),64)

			data["usage"] = utils.RoundFloatDigit2(convertData)
		}

		result = append(result, networkInEthError)
		result = append(result, networkInVxError)
		result = append(result, networkOutEthError)
		result = append(result, networkOutVxError)

		return result, nil
	}
}