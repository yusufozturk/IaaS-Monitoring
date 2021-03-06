package integration

import (
	"fmt"
	"io/ioutil"
	"encoding/json"
	"github.com/rackspace/gophercloud/openstack"
	"github.com/rackspace/gophercloud"
	"kr/paasta/monitoring/openstack/models"
	"kr/paasta/monitoring/openstack/utils"
)

type Neutron struct {
	OpenstackProvider 	models.OpenstackProvider
	Provider 		*gophercloud.ProviderClient
}

func GetNeutron(openstack_provider models.OpenstackProvider, provider *gophercloud.ProviderClient) *Neutron {
	return &Neutron{
		OpenstackProvider: 	openstack_provider,
		Provider: 		provider,
	}
}

/**
Description : Get project Network Limit metadata
 */
func (n *Neutron) GetProjectNetworkLimit(project_id string) (result models.ProjectNetworkLimit, err error){

	var data interface{}
	//client for Neutron API operation
	client, err := openstack.NewComputeV2(n.Provider, gophercloud.EndpointOpts{
		Region: n.OpenstackProvider.Region,
	})

	//Neutron Project Network Quota Information
	response, err := client.Get(fmt.Sprintf("%s/%s/quotas/%s", models.NeutronUrl, models.NeutronVersion, project_id), nil, nil)

	if err != nil {
		return result, err
	}
	rawdata, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return result, err
	}
	json.Unmarshal(rawdata, &data)
	msg := data.(map[string]interface{})
	resources := msg["quota"].(map[string]interface{})
	if len(resources) > 0 {
		result.RouterLimit = utils.TypeChecker_int(resources["router"]).(int)
		result.PortLimit = utils.TypeChecker_int(resources["port"]).(int)
		result.SecurityGroupRuleLimit = utils.TypeChecker_int(resources["security_group_rule"]).(int)
		result.SecurityGroupLimit = utils.TypeChecker_int(resources["security_group"]).(int)
		result.FloatingIpsLimit = utils.TypeChecker_int(resources["floatingip"]).(int)
		result.SubnetLimit = utils.TypeChecker_int(resources["subnet"]).(int)
		result.NetworkLimit = utils.TypeChecker_int(resources["network"]).(int)
	}
	return result, nil
}

/**
Description : Get project Generated Security Groups Information - Only return number of security groups.
 */
func (n *Neutron) GetProjectSecurityGroups(project_id string) (result int, err error){

	var data interface{}
	//client for Neutron API operation
	client, err := openstack.NewNetworkV2(n.Provider, gophercloud.EndpointOpts{
		Region: n.OpenstackProvider.Region,
	})

	//Neutron Project Floating IPs Information
	response, err := client.Get(fmt.Sprintf("%s/%s/security-groups.json?tenant_id=%s", models.NeutronUrl, models.NeutronVersion, project_id), nil, nil)
	if err != nil {
		return 0, err
	}
	rawdata, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return result, err
	}
	json.Unmarshal(rawdata, &data)
	msg := data.(map[string]interface{})
	resources := msg["security_groups"].([]interface{})
	/**
	It's complicated response. If you need details of security groups, parse response.
	Monitoring system just needs number of security groups .
	 */

	return len(resources), nil
}

/**
Description : Get project Generated Floating IPs Information
 */
func (n *Neutron) GetProjectFloatingIps(project_id string) (result []models.FloatingIPInfo, err error){

	var data interface{}
	//client for Neutron API operation
	client, err := openstack.NewComputeV2(n.Provider, gophercloud.EndpointOpts{
		Region: n.OpenstackProvider.Region,
	})

	//Neutron Project Floating IPs Information
	response, err := client.Get(fmt.Sprintf("%s/%s/floatingips.json?tenant_id=%s", models.NeutronUrl, models.NeutronVersion, project_id), nil, nil)
	if err != nil {
		return result, err
	}
	rawdata, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return result, err
	}
	json.Unmarshal(rawdata, &data)
	msg := data.(map[string]interface{})
	resources := msg["floatingips"].([]interface{})
	if(len(resources) > 0) {
		for _, v :=range resources{
			floatingip := v.(map[string]interface{})
			var floating_info models.FloatingIPInfo
			floating_info.RouterId   = utils.TypeChecker_string(floatingip["router_id"]).(string)
			floating_info.TenantId   = utils.TypeChecker_string(floatingip["tenant_id"]).(string)
			floating_info.FloatingNetworkId = utils.TypeChecker_string(floatingip["floating_network_id"]).(string)
			floating_info.InnerIp    = utils.TypeChecker_string(floatingip["fixed_ip_address"]).(string)
			floating_info.FloatingIp = utils.TypeChecker_string(floatingip["floating_ip_address"]).(string)
			floating_info.PortId     = utils.TypeChecker_string(floatingip["port_id"]).(string)
			floating_info.Status 	  = utils.TypeChecker_string(floatingip["status"]).(string)
			floating_info.Description = utils.TypeChecker_string(floatingip["description"]).(string)
			result = append(result, floating_info)
		}
	}
	return result, nil
}