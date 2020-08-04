//(C) Copyright [2020] Hewlett Packard Enterprise Development LP
//
//Licensed under the Apache License, Version 2.0 (the "License"); you may
//not use this file except in compliance with the License. You may obtain
//a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//License for the specific language governing permissions and limitations
// under the License.

// Package agmodel ...
package agmodel

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"

	dmtfmodel "github.com/ODIM-Project/ODIM/lib-dmtf/model"
	"github.com/ODIM-Project/ODIM/lib-utilities/common"
	"github.com/ODIM-Project/ODIM/lib-utilities/config"
	"github.com/ODIM-Project/ODIM/lib-utilities/errors"
)

//Schema model is used to iterate throgh the schema json for search/filter
type Schema struct {
	SearchKeys    []map[string]map[string]string `json:"searchKeys"`
	ConditionKeys []string                       `json:"conditionKeys"`
	QueryKeys     []string                       `json:"queryKeys"`
}

//SaveSystem model is used to save encrypted data into db
type SaveSystem struct {
	ManagerAddress string
	Password       []byte
	UserName       string
	DeviceUUID     string
	PluginID       string
}

// Plugin is the model for plugin information
type Plugin struct {
	IP                string
	Port              string
	Username          string
	Password          []byte
	ID                string
	PluginType        string
	PreferredAuthType string
}

//Target is for sending the requst to south bound/plugin
type Target struct {
	ManagerAddress string `json:"ManagerAddress"`
	Password       []byte `json:"Password"`
	UserName       string `json:"UserName"`
	PostBody       []byte `json:"PostBody"`
	DeviceUUID     string `json:"DeviceUUID"`
	PluginID       string `json:"PluginID"`
}

//SystemOperation hold the value system operation(InventoryRediscovery or Delete)
type SystemOperation struct {
	Operation string
}

// AggregationSource  payload of adding a AggregationSource
type AggregationSource struct {
	HostName string
	UserName string
	Password []byte
	Links    interface{}
}

//GetResource fetches a resource from database using table and key
func GetResource(Table, key string) (string, *errors.Error) {
	conn, err := common.GetDBConnection(common.InMemory)
	if err != nil {
		return "", errors.PackError(err.ErrNo(), err)
	}
	resourceData, err := conn.Read(Table, key)
	if err != nil {
		return "", errors.PackError(err.ErrNo(), "error while trying to get resource details: ", err.Error())
	}
	var resource string
	if errs := json.Unmarshal([]byte(resourceData), &resource); errs != nil {
		return "", errors.PackError(errors.UndefinedErrorType, errs)
	}
	return resource, nil
}

// Create connects to the persistencemgr and creates a system in db
func (system *SaveSystem) Create(systemID string) *errors.Error {

	conn, err := common.GetDBConnection(common.OnDisk)
	if err != nil {
		return err
	}
	//Create a header for data entry
	const table string = "System"
	//Save data into Database
	if err = conn.Create(table, systemID, system); err != nil {
		return err
	}
	return nil
}

//GetPluginData will fetch plugin details
func GetPluginData(pluginID string) (Plugin, *errors.Error) {
	var plugin Plugin

	conn, err := common.GetDBConnection(common.OnDisk)
	if err != nil {
		return plugin, err
	}

	plugindata, err := conn.Read("Plugin", pluginID)
	if err != nil {
		return plugin, errors.PackError(err.ErrNo(), "error while trying to fetch plugin data: ", err.Error())
	}

	if err := json.Unmarshal([]byte(plugindata), &plugin); err != nil {
		return plugin, errors.PackError(errors.JSONUnmarshalFailed, err)
	}

	bytepw, errs := common.DecryptWithPrivateKey([]byte(plugin.Password))
	if errs != nil {
		return Plugin{}, errors.PackError(errors.DecryptionFailed, "error: "+pluginID+" plugin password decryption failed: "+errs.Error())
	}
	plugin.Password = bytepw

	return plugin, nil
}

//GetComputeSystem will fetch the compute resource details
func GetComputeSystem(deviceUUID string) (dmtfmodel.ComputerSystem, error) {
	var compute dmtfmodel.ComputerSystem

	conn, err := common.GetDBConnection(common.InMemory)
	if err != nil {
		return compute, err
	}

	computeData, err := conn.Read("ComputerSystem", deviceUUID)
	if err != nil {
		return compute, fmt.Errorf("error while trying to get compute details: %v", err.Error())
	}

	if err := json.Unmarshal([]byte(computeData), &compute); err != nil {
		return compute, err
	}
	return compute, nil

}

//SaveComputeSystem will save the compute server complete details into the database
func SaveComputeSystem(computeServer dmtfmodel.ComputerSystem, deviceUUID string) error {
	//use dmtf logic to save data into database
	log.Println("Saving server details into database")
	err := computeServer.SaveInMemory(deviceUUID)
	if err != nil {
		return err
	}
	return nil
}

//SaveChassis will save the chassis details into the database
func SaveChassis(chassis dmtfmodel.Chassis, deviceUUID string) error {
	//use dmtf logic to save data into database
	log.Println("Saving server details into database")
	err := chassis.SaveInMemory(deviceUUID)
	if err != nil {
		return err
	}
	return nil
}

//GenericSave will save any resource data into the database
func GenericSave(body []byte, table string, key string) error {

	connPool, err := common.GetDBConnection(common.InMemory)
	if err != nil {
		return fmt.Errorf("error while trying to connecting to DB: %v", err.Error())
	}
	if err = connPool.AddResourceData(table, key, string(body)); err != nil {
		return fmt.Errorf("error while trying to create new %v resource: %v", table, err.Error())
	}
	return nil
}

//SaveRegistryFile will save any Registry file in database OnDisk DB
func SaveRegistryFile(body []byte, table string, key string) error {

	connPool, err := common.GetDBConnection(common.OnDisk)
	if err != nil {
		return fmt.Errorf("error while trying to connecting to DB: %v", err.Error())
	}
	if err = connPool.Create(table, key, string(body)); err != nil {
		if errors.DBKeyAlreadyExist != err.ErrNo() {
			return fmt.Errorf("error while trying to create new %v resource: %v", table, err.Error())
		}
		log.Printf("warning: skipped saving of duplicate data with key %v", key)
		return nil
	}
	return nil
}

//GetRegistryFile from Onisk DB
func GetRegistryFile(Table, key string) (string, *errors.Error) {
	conn, err := common.GetDBConnection(common.OnDisk)
	if err != nil {
		return "", errors.PackError(err.ErrNo(), err)
	}
	resourceData, err := conn.Read(Table, key)
	if err != nil {
		return "", errors.PackError(err.ErrNo(), "error while trying to get resource details: ", err.Error())
	}
	var resource string
	if errs := json.Unmarshal([]byte(resourceData), &resource); errs != nil {
		return "", errors.PackError(errors.UndefinedErrorType, errs)
	}
	return resource, nil
}

//DeleteComputeSystem will delete the compute system
func DeleteComputeSystem(index int, key string) *errors.Error {
	connPool, err := common.GetDBConnection(common.InMemory)
	if err != nil {
		return errors.PackError(err.ErrNo(), "error while trying to connecting to DB: ", err.Error())
	}

	// Check key present in the DB
	if _, err = connPool.Read("ComputerSystem", key); err != nil {
		return errors.PackError(err.ErrNo(), "error while trying to get compute details: ", err.Error())
	}

	//Delete All resources
	deleteKey := "*" + key[index+1:] + "*"
	if err = connPool.DeleteServer(deleteKey); err != nil {
		return errors.PackError(err.ErrNo(), "error while trying to delete compute system: ", err.Error())
	}
	if errs := deletefilteredkeys(key); errs != nil {
		return errors.PackError(errors.UndefinedErrorType, errs)
	}

	return nil
}

func deletefilteredkeys(key string) error {
	var sf Schema
	schemaFile, ioErr := ioutil.ReadFile(config.Data.SearchAndFilterSchemaPath)
	if ioErr != nil {
		return fmt.Errorf("fatal: error while trying to read search/filter schema json: %v", ioErr)
	}
	jsonErr := json.Unmarshal(schemaFile, &sf)
	if jsonErr != nil {
		return fmt.Errorf("fatal: error while trying to fetch search/filter schema json: %v", jsonErr)
	}
	conn, err := common.GetDBConnection(common.InMemory)
	if err != nil {
		return fmt.Errorf("error while trying to connecting to DB: %v", err)
	}
	for _, value := range sf.SearchKeys {
		for k := range value {
			delErr := conn.Del(k, key)
			if delErr != nil {
				if delErr.Error() != "no data with ID found" {
					return fmt.Errorf("error while deleting data: " + delErr.Error())
				}
			}
		}
	}
	delErr := conn.Del("UUID", key)
	if delErr != nil {
		if delErr.Error() != "no data with ID found" {
			return fmt.Errorf("error while deleting data: " + delErr.Error())
		}
	}
	delErr = conn.Del("PowerState", key)
	if delErr != nil {
		if delErr.Error() != "no data with ID found" {
			return fmt.Errorf("error while deleting data: " + delErr.Error())
		}
	}
	return nil
}

//DeleteSystem will delete the system from OnDisk
func DeleteSystem(key string) *errors.Error {
	connPool, err := common.GetDBConnection(common.OnDisk)
	if err != nil {
		return errors.PackError(err.ErrNo(), "error while trying to connecting to DB: ", err.Error())
	}

	// Check key present in the DB
	if _, err = connPool.Read("System", key); err != nil {
		return errors.PackError(err.ErrNo(), "error while trying to get compute details: ", err.Error())
	}

	deleteKey := "System:" + key
	//Delete All resources
	if err = connPool.DeleteServer(deleteKey); err != nil {
		return errors.PackError(err.ErrNo(), "error while trying to delete compute system: ", err.Error())
	}
	return nil
}

//GetTarget fetches the System(Target Device Credentials) table details
func GetTarget(deviceUUID string) (*Target, error) {
	var target Target
	conn, err := common.GetDBConnection(common.OnDisk)
	if err != nil {
		return nil, err
	}

	data, err := conn.Read("System", deviceUUID)
	if err != nil {
		return nil, fmt.Errorf("error while trying to get compute details: %v", err.Error())
	}

	if err := json.Unmarshal([]byte(data), &target); err != nil {
		return nil, err
	}

	return &target, nil
}

//SaveIndex is used to create a
func SaveIndex(searchForm map[string]interface{}, table, uuid string) error {
	conn, err := common.GetDBConnection(common.InMemory)
	if err != nil {
		return fmt.Errorf("error while trying to connecting to DB: %v", err)
	}
	log.Println("Creating index")
	searchForm["UUID"] = uuid
	if err := conn.CreateIndex(searchForm, table); err != nil {
		return fmt.Errorf("error while trying to index the document: %v", err)
	}

	return nil

}

//SavePluginData will saves plugin on disk
func SavePluginData(plugin Plugin) *errors.Error {

	conn, err := common.GetDBConnection(common.OnDisk)
	if err != nil {
		return err
	}
	const table string = "Plugin"
	if err := conn.Create(table, plugin.ID, plugin); err != nil {
		return errors.PackError(err.ErrNo(), "error while trying to save plugin data: ", err.Error())
	}

	return nil
}

// GetAllSystems extracts all the computer systems saved in ondisk
func GetAllSystems() ([]Target, *errors.Error) {
	conn, err := common.GetDBConnection(common.OnDisk)
	if err != nil {
		return nil, err
	}
	keys, err := conn.GetAllDetails("System")
	if err != nil {
		return nil, err
	}
	var targets []Target
	for _, key := range keys {
		var target Target
		targetData, err := conn.Read("System", key)
		if err != nil {
			return nil, err
		}
		if errs := json.Unmarshal([]byte(targetData), &target); errs != nil {
			return nil, errors.PackError(errors.UndefinedErrorType, errs)
		}
		targets = append(targets, target)

	}
	return targets, nil
}

//DeletePluginData will delete the plugin entry from the database based on the uuid
func DeletePluginData(key string) *errors.Error {
	conn, err := common.GetDBConnection(common.OnDisk)
	if err != nil {
		return err
	}
	if err = conn.Delete("Plugin", key); err != nil {
		return err
	}
	return nil
}

//DeleteManagersData will delete the Managers entry from the database based on the uuid
func DeleteManagersData(key string) *errors.Error {
	conn, err := common.GetDBConnection(common.InMemory)
	if err != nil {
		return err
	}
	if err = conn.Delete("Managers", key); err != nil {
		return err
	}
	return nil
}

//UpdateIndex is used for updating an existing index
func UpdateIndex(searchForm map[string]interface{}, table, uuid string) error {
	conn, err := common.GetDBConnection(common.InMemory)
	if err != nil {
		return fmt.Errorf("error while trying to connecting to DB: %v", err)
	}
	searchForm["UUID"] = uuid
	if err := conn.UpdateResourceIndex(searchForm, table); err != nil {
		return fmt.Errorf("error while trying to update index: %v", err)
	}

	return nil
}

//UpdateComputeSystem is used for updating ComputerSystem table
func UpdateComputeSystem(key string, computeData interface{}) error {
	conn, err := common.GetDBConnection(common.InMemory)
	if err != nil {
		return err
	}
	marshaledData, errs := json.Marshal(computeData)
	if errs != nil {
		return errs
	}
	if _, err := conn.Update("ComputerSystem", key, string(marshaledData)); err != nil {
		return err
	}
	return nil
}

//GetResourceDetails fetches a resource from database using key
func GetResourceDetails(key string) (string, *errors.Error) {
	conn, err := common.GetDBConnection(common.InMemory)
	if err != nil {
		return "", errors.PackError(err.ErrNo(), err)
	}
	resourceData, err := conn.GetResourceDetails(key)
	if err != nil {
		return "", errors.PackError(err.ErrNo(), "error while trying to get resource details: ", err.Error())
	}
	var resource string
	if errs := json.Unmarshal([]byte(resourceData), &resource); errs != nil {
		return "", errors.PackError(errors.UndefinedErrorType, errs)
	}
	return resource, nil
}

// GetString is used to retrive index values of type string
/* Inputs:
1. index is the index name to search with
2. match is the value to match with
*/
func GetString(index, match string) ([]string, error) {
	conn, dberr := common.GetDBConnection(common.InMemory)
	if dberr != nil {
		return nil, fmt.Errorf("error while trying to connecting to DB: %v", dberr.Error())
	}
	list, err := conn.GetString(index, 0, "*"+match+"*", false)
	if err != nil && err.Error() != "no data with ID found" {
		fmt.Println("error while getting the data", err)
		return []string{}, nil
	}
	return list, nil
}

// AddSystemOperationInfo connects to the persistencemgr and Add the system operation info to db
/* Inputs:
1.systemURI: computer system uri for which system operation is maintained
*/
func (system *SystemOperation) AddSystemOperationInfo(systemID string) *errors.Error {

	conn, err := common.GetDBConnection(common.InMemory)
	if err != nil {
		return err
	}
	//Create a header for data entry
	const table string = "SystemOperation"
	//Save data into Database
	if err = conn.AddResourceData(table, systemID, system); err != nil {
		return err
	}
	return nil
}

//GetSystemOperationInfo fetches the system opeation info for the given systemURI
/* Inputs:
1.systemURI: computer system uri for which system operation is maintained
*/
func GetSystemOperationInfo(systemURI string) (SystemOperation, *errors.Error) {
	var systemOperation SystemOperation

	conn, err := common.GetDBConnection(common.InMemory)
	if err != nil {
		return systemOperation, err
	}

	plugindata, err := conn.Read("SystemOperation", systemURI)
	if err != nil {
		return systemOperation, errors.PackError(err.ErrNo(), "error while trying to fetch system operation data: ", err.Error())
	}

	if err := json.Unmarshal([]byte(plugindata), &systemOperation); err != nil {
		return systemOperation, errors.PackError(errors.JSONUnmarshalFailed, err)
	}
	return systemOperation, nil
}

//DeleteSystemOperationInfo will delete the system operation entry from the database based on the systemURI
func DeleteSystemOperationInfo(systemURI string) *errors.Error {
	conn, err := common.GetDBConnection(common.InMemory)
	if err != nil {
		return err
	}
	if err = conn.Delete("SystemOperation", systemURI); err != nil {
		return err
	}
	return nil
}

// AddSystemResetInfo connects to the persistencemgr and Add the system reset info to db
/* Inputs:
1.systemURI: computer system uri for which system operation is maintained
2.resetType : reset type which is performed
*/
func AddSystemResetInfo(systemID, resetType string) *errors.Error {

	conn, err := common.GetDBConnection(common.InMemory)
	if err != nil {
		return err
	}
	//Create a header for data entry
	const table string = "SystemReset"
	//Save data into Database
	if err = conn.AddResourceData(table, systemID, map[string]string{
		"ResetType": resetType,
	}); err != nil {
		return err
	}
	return nil
}

//GetSystemResetInfo fetches the system reset info for the given systemURI
/* Inputs:
1.systemURI: computer system uri for which system operation is maintained
*/
func GetSystemResetInfo(systemURI string) (map[string]string, *errors.Error) {
	var resetInfo map[string]string

	conn, err := common.GetDBConnection(common.InMemory)
	if err != nil {
		return resetInfo, err
	}

	plugindata, err := conn.Read("SystemReset", systemURI)
	if err != nil {
		return resetInfo, errors.PackError(err.ErrNo(), "error while trying to fetch system reset data: ", err.Error())
	}

	if err := json.Unmarshal([]byte(plugindata), &resetInfo); err != nil {
		return resetInfo, errors.PackError(errors.JSONUnmarshalFailed, err)
	}
	return resetInfo, nil
}

//DeleteSystemResetInfo will delete the system reset entry from the database based on the systemURI
func DeleteSystemResetInfo(systemURI string) *errors.Error {
	conn, err := common.GetDBConnection(common.InMemory)
	if err != nil {
		return err
	}
	if err = conn.Delete("SystemReset", systemURI); err != nil {
		return err
	}
	return nil
}

// AddAggregationSource connects to the persistencemgr and Add the AggregationSource to db
/* Inputs:
1.req: AggregationSource info
2.aggregationSourceURI : uri of AggregationSource
*/
func AddAggregationSource(req AggregationSource, aggregationSourceURI string) *errors.Error {
	conn, err := common.GetDBConnection(common.OnDisk)
	if err != nil {
		return err
	}
	//Create a header for data entry
	const table string = "AggregationSource"
	//Save data into Database
	if err = conn.Create(table, aggregationSourceURI, req); err != nil {
		return err
	}
	return nil
}

// GetAggregationSourceInfo fetches the AggregationSource info for the given aggregationSourceURI
func GetAggregationSourceInfo(aggregationSourceURI string) (AggregationSource, *errors.Error) {
	var aggregationSource AggregationSource

	conn, err := common.GetDBConnection(common.OnDisk)
	if err != nil {
		return aggregationSource, err
	}

	data, err := conn.Read("AggregationSource", aggregationSourceURI)
	if err != nil {
		return aggregationSource, errors.PackError(err.ErrNo(), "error: while trying to fetch connection method data: ", err.Error())
	}

	if err := json.Unmarshal([]byte(data), &aggregationSource); err != nil {
		return aggregationSource, errors.PackError(errors.JSONUnmarshalFailed, err)
	}
	return aggregationSource, nil
}