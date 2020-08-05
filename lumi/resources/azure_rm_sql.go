package resources

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/sql/mgmt/sql"
	"github.com/rs/zerolog/log"
)

func (a *lumiAzurermSqlConfiguration) id() (string, error) {
	return a.Id()
}

func (a *lumiAzurermSqlFirewallrule) id() (string, error) {
	return a.Id()
}

func (a *lumiAzurerm) GetSqlServers() ([]interface{}, error) {
	at, err := azuretransport(a.Runtime.Motor.Transport)
	if err != nil {
		return nil, err
	}

	subscriptionID := at.SubscriptionID()

	ctx := context.Background()
	authorizer, err := at.Authorizer()
	if err != nil {
		return nil, err
	}

	dbClient := sql.NewServersClient(subscriptionID)
	dbClient.Authorizer = authorizer

	servers, err := dbClient.List(ctx)
	if err != nil {
		return nil, err
	}

	res := []interface{}{}
	if servers.Value == nil {
		return res, nil
	}

	dbServers := *servers.Value

	for i := range dbServers {
		dbServer := dbServers[i]

		properties := make(map[string](interface{}))

		data, err := json.Marshal(dbServer.ServerProperties)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(data), &properties)
		if err != nil {
			return nil, err
		}

		lumiAzureDbServer, err := a.Runtime.CreateResource("azurerm.sql.server",
			"id", toString(dbServer.ID),
			"name", toString(dbServer.Name),
			"location", toString(dbServer.Location),
			"tags", azureTagsToInterface(dbServer.Tags),
			"type", toString(dbServer.Type),
			"properties", properties,
		)
		if err != nil {
			return nil, err
		}
		res = append(res, lumiAzureDbServer)
	}

	return res, nil
}

func (a *lumiAzurermSqlServer) id() (string, error) {
	return a.Id()
}

func (a *lumiAzurermSqlServer) GetDatabases() ([]interface{}, error) {
	at, err := azuretransport(a.Runtime.Motor.Transport)
	if err != nil {
		return nil, err
	}

	// id is a azure resource od
	id, err := a.Id()
	if err != nil {
		return nil, err
	}

	resourceID, err := at.ParseResourceID(id)
	if err != nil {
		return nil, err
	}

	server, err := resourceID.Component("servers")
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	authorizer, err := at.Authorizer()
	if err != nil {
		return nil, err
	}

	dbDatabaseClient := sql.NewDatabasesClient(resourceID.SubscriptionID)
	dbDatabaseClient.Authorizer = authorizer

	databases, err := dbDatabaseClient.ListByServer(ctx, resourceID.ResourceGroup, server, "", "")
	if err != nil {
		return nil, err
	}

	res := []interface{}{}

	if databases.Value == nil {
		return res, nil
	}

	list := *databases.Value
	for i := range list {
		entry := list[i]

		transparentDataEncryption := make(map[string](interface{}))
		data, err := json.Marshal(entry.TransparentDataEncryption)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal([]byte(data), &transparentDataEncryption)
		if err != nil {
			return nil, err
		}

		recommendedIndex := make(map[string](interface{}))
		data, err = json.Marshal(entry.RecommendedIndex)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal([]byte(data), &recommendedIndex)
		if err != nil {
			return nil, err
		}

		serviceTierAdvisors := make(map[string](interface{}))
		data, err = json.Marshal(entry.ServiceTierAdvisors)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal([]byte(data), &serviceTierAdvisors)
		if err != nil {
			return nil, err
		}

		lumiAzureDatabase, err := a.Runtime.CreateResource("azurerm.sql.database",
			"id", toString(entry.ID),
			"name", toString(entry.Name),
			"type", toString(entry.Type),
			"collation", toString(entry.Collation),
			"creationDate", azureRmTime(entry.CreationDate),
			"containmentState", toInt64(entry.ContainmentState),
			"currentServiceObjectiveId", uuidToString(entry.CurrentServiceObjectiveID),
			"databaseId", uuidToString(entry.DatabaseID),
			"earliestRestoreDate", azureRmTime(entry.EarliestRestoreDate),
			"createMode", string(entry.CreateMode),
			"sourceDatabaseId", toString(entry.SourceDatabaseID),
			"sourceDatabaseDeletionDate", azureRmTime(entry.SourceDatabaseDeletionDate),
			"restorePointInTime", azureRmTime(entry.RestorePointInTime),
			"recoveryServicesRecoveryPointResourceId", toString(entry.RecoveryServicesRecoveryPointResourceID),
			"edition", string(entry.Edition),
			"maxSizeBytes", toString(entry.MaxSizeBytes),
			"requestedServiceObjectiveId", uuidToString(entry.RequestedServiceObjectiveID),
			"requestedServiceObjectiveName", string(entry.RequestedServiceObjectiveName),
			"serviceLevelObjective", string(entry.ServiceLevelObjective),
			"status", toString(entry.Status),
			"elasticPoolName", toString(entry.ElasticPoolName),
			"defaultSecondaryLocation", toString(entry.DefaultSecondaryLocation),
			"serviceTierAdvisors", serviceTierAdvisors,
			"transparentDataEncryption", transparentDataEncryption,
			"recommendedIndex", recommendedIndex,
			"failoverGroupId", toString(entry.FailoverGroupID),
			"readScale", string(entry.ReadScale),
			"sampleName", string(entry.SampleName),
			"zoneRedundant", toBool(entry.ZoneRedundant),
		)
		if err != nil {
			return nil, err
		}
		res = append(res, lumiAzureDatabase)
	}

	return res, nil
}

func (a *lumiAzurermSqlServer) GetFirewallRules() ([]interface{}, error) {
	at, err := azuretransport(a.Runtime.Motor.Transport)
	if err != nil {
		return nil, err
	}

	// id is a azure resource od
	id, err := a.Id()
	if err != nil {
		return nil, err
	}

	resourceID, err := at.ParseResourceID(id)
	if err != nil {
		return nil, err
	}

	server, err := resourceID.Component("servers")
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	authorizer, err := at.Authorizer()
	if err != nil {
		return nil, err
	}

	dbFirewallClient := sql.NewFirewallRulesClient(resourceID.SubscriptionID)
	dbFirewallClient.Authorizer = authorizer

	firewallRules, err := dbFirewallClient.ListByServer(ctx, resourceID.ResourceGroup, server)
	if err != nil {
		return nil, err
	}

	res := []interface{}{}

	if firewallRules.Value == nil {
		return res, nil
	}

	list := *firewallRules.Value
	for i := range list {
		entry := list[i]

		lumiAzureConfiguration, err := a.Runtime.CreateResource("azurerm.sql.firewallrule",
			"id", toString(entry.ID),
			"name", toString(entry.Name),
			"type", toString(entry.Type),
			"startIpAddress", toString(entry.StartIPAddress),
			"endIpAddress", toString(entry.EndIPAddress),
		)
		if err != nil {
			return nil, err
		}
		res = append(res, lumiAzureConfiguration)
	}

	return res, nil
}

func (a *lumiAzurermSqlServer) GetAdministrators() ([]interface{}, error) {
	at, err := azuretransport(a.Runtime.Motor.Transport)
	if err != nil {
		return nil, err
	}

	// id is a azure resource od
	id, err := a.Id()
	if err != nil {
		return nil, err
	}

	resourceID, err := at.ParseResourceID(id)
	if err != nil {
		return nil, err
	}

	server, err := resourceID.Component("servers")
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	authorizer, err := at.Authorizer()
	if err != nil {
		return nil, err
	}

	administratorClient := sql.NewServerAzureADAdministratorsClient(resourceID.SubscriptionID)
	administratorClient.Authorizer = authorizer

	administrators, err := administratorClient.ListByServer(ctx, resourceID.ResourceGroup, server)
	if err != nil {
		return nil, err
	}

	res := []interface{}{}

	if administrators.Value == nil {
		return res, nil
	}

	list := *administrators.Value
	for i := range list {
		entry := list[i]

		lumiAzureSqlAdministrator, err := a.Runtime.CreateResource("azurerm.sql.server.administrator",
			"id", toString(entry.ID),
			"name", toString(entry.Name),
			"type", toString(entry.Type),
			"administratorType", toString(entry.AdministratorType),
			"login", toString(entry.Login),
			"sid", uuidToString(entry.Sid),
			"tenantId", uuidToString(entry.TenantID),
		)
		if err != nil {
			return nil, err
		}
		res = append(res, lumiAzureSqlAdministrator)
	}

	return res, nil
}

func (a *lumiAzurermSqlServer) GetConnectionPolicy() (map[string]interface{}, error) {
	at, err := azuretransport(a.Runtime.Motor.Transport)
	if err != nil {
		return nil, err
	}

	// id is a azure resource od
	id, err := a.Id()
	if err != nil {
		return nil, err
	}

	resourceID, err := at.ParseResourceID(id)
	if err != nil {
		return nil, err
	}

	server, err := resourceID.Component("servers")
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	authorizer, err := at.Authorizer()
	if err != nil {
		return nil, err
	}

	connectionClient := sql.NewServerConnectionPoliciesClient(resourceID.SubscriptionID)
	connectionClient.Authorizer = authorizer

	policy, err := connectionClient.Get(ctx, resourceID.ResourceGroup, server)
	if err != nil {
		return nil, err
	}

	return jsonToDict(policy)
}

func (a *lumiAzurermSqlDatabase) id() (string, error) {
	return a.Id()
}

func (a *lumiAzurermSqlDatabase) GetUsage() ([]interface{}, error) {
	at, err := azuretransport(a.Runtime.Motor.Transport)
	if err != nil {
		return nil, err
	}

	// id is a azure resource od
	id, err := a.Id()
	if err != nil {
		return nil, err
	}

	resourceID, err := at.ParseResourceID(id)
	if err != nil {
		return nil, err
	}

	server, err := resourceID.Component("servers")
	if err != nil {
		return nil, err
	}

	database, err := resourceID.Component("databases")
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	authorizer, err := at.Authorizer()
	if err != nil {
		return nil, err
	}

	client := sql.NewDatabaseUsagesClient(resourceID.SubscriptionID)
	client.Authorizer = authorizer

	usage, err := client.ListByDatabase(ctx, resourceID.ResourceGroup, server, database)
	if err != nil {
		return nil, err
	}

	res := []interface{}{}

	if usage.Value == nil {
		return res, nil
	}

	list := *usage.Value

	for i := range list {
		entry := list[i]

		fmt.Printf("%v\n", entry)

		lumiAzureSqlUsage, err := a.Runtime.CreateResource("azurerm.sql.databaseusage",
			"id", id+"/metrics/"+toString(entry.Name),
			"name", toString(entry.Name),
			"resourceName", toString(entry.ResourceName),
			"displayName", toString(entry.DisplayName),
			"currentValue", toFloat64(entry.CurrentValue),
			"limit", toFloat64(entry.Limit),
			"unit", toString(entry.Unit),
			"nextResetTime", azureRmTime(entry.NextResetTime),
		)
		if err != nil {
			log.Error().Err(err).Msg("could not create lumi resource")
			return nil, err
		}
		res = append(res, lumiAzureSqlUsage)
	}

	return res, nil
}

func (a *lumiAzurermSqlDatabaseusage) id() (string, error) {
	return a.Id()
}

func (a *lumiAzurermSqlDatabase) GetAdvisor() ([]interface{}, error) {
	at, err := azuretransport(a.Runtime.Motor.Transport)
	if err != nil {
		return nil, err
	}

	// id is a azure resource od
	id, err := a.Id()
	if err != nil {
		return nil, err
	}

	resourceID, err := at.ParseResourceID(id)
	if err != nil {
		return nil, err
	}

	server, err := resourceID.Component("servers")
	if err != nil {
		return nil, err
	}

	database, err := resourceID.Component("databases")
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	authorizer, err := at.Authorizer()
	if err != nil {
		return nil, err
	}

	client := sql.NewDatabaseAdvisorsClient(resourceID.SubscriptionID)
	client.Authorizer = authorizer

	advisors, err := client.ListByDatabase(ctx, resourceID.ResourceGroup, server, database)
	if err != nil {
		return nil, err
	}

	res := []interface{}{}

	if advisors.Value == nil {
		return res, nil
	}

	list := *advisors.Value

	for i := range list {
		entry := list[i]

		dict, err := jsonToDict(entry)
		if err != nil {
			return nil, err
		}

		res = append(res, dict)
	}

	return res, nil
}

func (a *lumiAzurermSqlDatabase) GetThreadDetectionPolicy() (map[string]interface{}, error) {
	at, err := azuretransport(a.Runtime.Motor.Transport)
	if err != nil {
		return nil, err
	}

	// id is a azure resource od
	id, err := a.Id()
	if err != nil {
		return nil, err
	}

	resourceID, err := at.ParseResourceID(id)
	if err != nil {
		return nil, err
	}

	server, err := resourceID.Component("servers")
	if err != nil {
		return nil, err
	}

	database, err := resourceID.Component("databases")
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	authorizer, err := at.Authorizer()
	if err != nil {
		return nil, err
	}

	client := sql.NewDatabaseThreatDetectionPoliciesClient(resourceID.SubscriptionID)
	client.Authorizer = authorizer

	policy, err := client.Get(ctx, resourceID.ResourceGroup, server, database)
	if err != nil {
		return nil, err
	}

	return jsonToDict(policy)
}

func (a *lumiAzurermSqlDatabase) GetConnectionPolicy() (map[string]interface{}, error) {
	at, err := azuretransport(a.Runtime.Motor.Transport)
	if err != nil {
		return nil, err
	}

	// id is a azure resource od
	id, err := a.Id()
	if err != nil {
		return nil, err
	}

	resourceID, err := at.ParseResourceID(id)
	if err != nil {
		return nil, err
	}

	server, err := resourceID.Component("servers")
	if err != nil {
		return nil, err
	}

	database, err := resourceID.Component("databases")
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	authorizer, err := at.Authorizer()
	if err != nil {
		return nil, err
	}

	connectionClient := sql.NewDatabaseConnectionPoliciesClient(resourceID.SubscriptionID)
	connectionClient.Authorizer = authorizer

	policy, err := connectionClient.Get(ctx, resourceID.ResourceGroup, server, database)
	if err != nil {
		return nil, err
	}

	return jsonToDict(policy)
}

func (a *lumiAzurermSqlServerAdministrator) id() (string, error) {
	return a.Id()
}