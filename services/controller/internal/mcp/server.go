// Package mcp provides an MCP (Model Context Protocol) server exposed as an HTTP endpoint.
// It exposes Controller Connect RPC operations as MCP tools.
package mcp

import (
	"context"
	"encoding/json"
	"net/http"

	"connectrpc.com/connect"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	icehivev1 "github.com/icehive/icehive/services/common/pkg/gen/icehive/v1"
)

// ControllerAPI is the subset of ControllerService methods exposed as MCP tools.
type ControllerAPI interface {
	Init(context.Context, *connect.Request[icehivev1.InitRequest]) (*connect.Response[icehivev1.InitResponse], error)
	Health(context.Context, *connect.Request[icehivev1.HealthRequest]) (*connect.Response[icehivev1.HealthResponse], error)
	ListServices(context.Context, *connect.Request[icehivev1.ListServicesRequest]) (*connect.Response[icehivev1.ListServicesResponse], error)
	ListActivity(context.Context, *connect.Request[icehivev1.ListActivityRequest]) (*connect.Response[icehivev1.ListActivityResponse], error)
	ListConfig(context.Context, *connect.Request[icehivev1.ListConfigRequest]) (*connect.Response[icehivev1.ListConfigResponse], error)
	GetConfig(context.Context, *connect.Request[icehivev1.GetConfigRequest]) (*connect.Response[icehivev1.GetConfigResponse], error)
	SetConfig(context.Context, *connect.Request[icehivev1.SetConfigRequest]) (*connect.Response[icehivev1.SetConfigResponse], error)
	ListCollectionSources(context.Context, *connect.Request[icehivev1.ListCollectionSourcesRequest]) (*connect.Response[icehivev1.ListCollectionSourcesResponse], error)
	ListCollectorSourceSchemas(context.Context, *connect.Request[icehivev1.ListCollectorSourceSchemasRequest]) (*connect.Response[icehivev1.ListCollectorSourceSchemasResponse], error)
	UpsertCollectionSource(context.Context, *connect.Request[icehivev1.UpsertCollectionSourceRequest]) (*connect.Response[icehivev1.UpsertCollectionSourceResponse], error)
	DeleteCollectionSource(context.Context, *connect.Request[icehivev1.DeleteCollectionSourceRequest]) (*connect.Response[icehivev1.DeleteCollectionSourceResponse], error)
	EnqueueCollectionRequest(context.Context, *connect.Request[icehivev1.EnqueueCollectionRequestRequest]) (*connect.Response[icehivev1.EnqueueCollectionRequestResponse], error)
}

// NewHandler returns an http.Handler that serves the MCP Streamable HTTP endpoint at /mcp.
func NewHandler(api ControllerAPI) http.Handler {
	mcpServer := server.NewMCPServer(
		"IceHive",
		"1.0.0",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)

	mcpServer.AddTool(mcp.NewTool("icehive_init",
		mcp.WithDescription("Confirm connectivity to the IceHive Controller and return the server version."),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInit(ctx, api)
	})

	mcpServer.AddTool(mcp.NewTool("icehive_health",
		mcp.WithDescription("Check Controller health (including metadata DB reachability)."),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleHealth(ctx, api)
	})

	mcpServer.AddTool(mcp.NewTool("icehive_list_services",
		mcp.WithDescription("List AMQP-connected services and their latest heartbeat status (healthy/stale/unknown)."),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleListServices(ctx, api)
	})

	mcpServer.AddTool(mcp.NewTool("icehive_list_activity",
		mcp.WithDescription("List recent Controller control-plane activity (heartbeats, collection enqueues, collection runs)."),
		mcp.WithNumber("limit", mcp.Description("Max events to return (default 50, max 100)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleListActivity(ctx, api, req)
	})

	mcpServer.AddTool(mcp.NewTool("icehive_list_config",
		mcp.WithDescription("List Controller configuration variables. Secret-looking keys are redacted."),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleListConfig(ctx, api)
	})

	mcpServer.AddTool(mcp.NewTool("icehive_get_config",
		mcp.WithDescription("Get a single Controller configuration variable by key."),
		mcp.WithString("key", mcp.Required(), mcp.Description("Flattened configuration key")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleGetConfig(ctx, api, req)
	})

	mcpServer.AddTool(mcp.NewTool("icehive_set_config",
		mcp.WithDescription("Set a Controller configuration variable and persist it."),
		mcp.WithString("key", mcp.Required(), mcp.Description("Flattened configuration key")),
		mcp.WithString("value", mcp.Required(), mcp.Description("New value")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleSetConfig(ctx, api, req)
	})

	mcpServer.AddTool(mcp.NewTool("icehive_list_collection_sources",
		mcp.WithDescription("List collection sources (collector targets). Optionally filter by collector_type."),
		mcp.WithString("collector_type", mcp.Description("Optional collector type filter, e.g. collector-github")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleListCollectionSources(ctx, api, req)
	})

	mcpServer.AddTool(mcp.NewTool("icehive_list_collector_source_schemas",
		mcp.WithDescription("List SourceSchema JSON documents published by collectors (for form discovery)."),
		mcp.WithString("collector_type", mcp.Description("Optional collector type filter")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleListCollectorSourceSchemas(ctx, api, req)
	})

	mcpServer.AddTool(mcp.NewTool("icehive_upsert_collection_source",
		mcp.WithDescription("Create or update a collection source. Pass source as a JSON object matching CollectionSource (id empty to create)."),
		mcp.WithString("source", mcp.Required(), mcp.Description("JSON object for CollectionSource fields")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleUpsertCollectionSource(ctx, api, req)
	})

	mcpServer.AddTool(mcp.NewTool("icehive_delete_collection_source",
		mcp.WithDescription("Delete a collection source by id."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Collection source ID")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDeleteCollectionSource(ctx, api, req)
	})

	mcpServer.AddTool(mcp.NewTool("icehive_enqueue_collection_request",
		mcp.WithDescription("Enqueue an immediate collection run. Provide collection_source_id for a persisted source, or ephemeral_collection JSON for a one-off run."),
		mcp.WithString("collection_source_id", mcp.Description("Persisted collection source ID")),
		mcp.WithString("ephemeral_collection", mcp.Description("JSON CollectionSource for a one-off run (not persisted)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleEnqueueCollectionRequest(ctx, api, req)
	})

	return server.NewStreamableHTTPServer(mcpServer, server.WithEndpointPath("/mcp"))
}

func jsonResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

func handleInit(ctx context.Context, api ControllerAPI) (*mcp.CallToolResult, error) {
	res, err := api.Init(ctx, connect.NewRequest(&icehivev1.InitRequest{}))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{"version": res.Msg.GetVersion()})
}

func handleHealth(ctx context.Context, api ControllerAPI) (*mcp.CallToolResult, error) {
	res, err := api.Health(ctx, connect.NewRequest(&icehivev1.HealthRequest{}))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{"status": res.Msg.GetStatus()})
}

func handleListServices(ctx context.Context, api ControllerAPI) (*mcp.CallToolResult, error) {
	res, err := api.ListServices(ctx, connect.NewRequest(&icehivev1.ListServicesRequest{}))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{"services": res.Msg.GetServices()})
}

func handleListActivity(ctx context.Context, api ControllerAPI, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	limit := int32(req.GetFloat("limit", 50))
	res, err := api.ListActivity(ctx, connect.NewRequest(&icehivev1.ListActivityRequest{Limit: limit}))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{"events": res.Msg.GetEvents()})
}

func handleListConfig(ctx context.Context, api ControllerAPI) (*mcp.CallToolResult, error) {
	res, err := api.ListConfig(ctx, connect.NewRequest(&icehivev1.ListConfigRequest{}))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{"vars": res.Msg.GetVars()})
}

func handleGetConfig(ctx context.Context, api ControllerAPI, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	key, err := req.RequireString("key")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	res, err := api.GetConfig(ctx, connect.NewRequest(&icehivev1.GetConfigRequest{Key: key}))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{"var": res.Msg.GetVar()})
}

func handleSetConfig(ctx context.Context, api ControllerAPI, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	key, err := req.RequireString("key")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	value, err := req.RequireString("value")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	_, err = api.SetConfig(ctx, connect.NewRequest(&icehivev1.SetConfigRequest{Key: key, Value: value}))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{"ok": true})
}

func handleListCollectionSources(ctx context.Context, api ControllerAPI, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	r := &icehivev1.ListCollectionSourcesRequest{
		CollectorType: req.GetString("collector_type", ""),
	}
	res, err := api.ListCollectionSources(ctx, connect.NewRequest(r))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{"sources": res.Msg.GetSources()})
}

func handleListCollectorSourceSchemas(ctx context.Context, api ControllerAPI, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	r := &icehivev1.ListCollectorSourceSchemasRequest{
		CollectorType: req.GetString("collector_type", ""),
	}
	res, err := api.ListCollectorSourceSchemas(ctx, connect.NewRequest(r))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{"schemas": res.Msg.GetSchemas()})
}

func handleUpsertCollectionSource(ctx context.Context, api ControllerAPI, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	raw, err := req.RequireString("source")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	var source icehivev1.CollectionSource
	parseMsg := ""
	if parseErr := json.Unmarshal([]byte(raw), &source); parseErr != nil {
		parseMsg = parseErr.Error()
	}
	if parseMsg != "" {
		return mcp.NewToolResultError("invalid source JSON: " + parseMsg), nil
	}
	res, err := api.UpsertCollectionSource(ctx, connect.NewRequest(&icehivev1.UpsertCollectionSourceRequest{Source: &source}))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{"source": res.Msg.GetSource()})
}

func handleDeleteCollectionSource(ctx context.Context, api ControllerAPI, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := req.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	_, err = api.DeleteCollectionSource(ctx, connect.NewRequest(&icehivev1.DeleteCollectionSourceRequest{Id: id}))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{"ok": true})
}

func enqueueRequestFromID(id string) *icehivev1.EnqueueCollectionRequestRequest {
	return &icehivev1.EnqueueCollectionRequestRequest{
		Target: &icehivev1.EnqueueCollectionRequestRequest_CollectionSourceId{CollectionSourceId: id},
	}
}

func enqueueRequestFromEphemeralJSON(raw string) (*icehivev1.EnqueueCollectionRequestRequest, string) {
	var source icehivev1.CollectionSource
	parseMsg := ""
	if parseErr := json.Unmarshal([]byte(raw), &source); parseErr != nil {
		parseMsg = parseErr.Error()
	}
	if parseMsg != "" {
		return nil, "invalid ephemeral_collection JSON: " + parseMsg
	}
	return &icehivev1.EnqueueCollectionRequestRequest{
		Target: &icehivev1.EnqueueCollectionRequestRequest_EphemeralCollection{EphemeralCollection: &source},
	}, ""
}

//gocyclo:ignore
func parseEnqueueTarget(req mcp.CallToolRequest) (*icehivev1.EnqueueCollectionRequestRequest, string) {
	id := req.GetString("collection_source_id", "")
	ephemeral := req.GetString("ephemeral_collection", "")
	switch {
	case id == "" && ephemeral == "":
		return nil, "provide collection_source_id or ephemeral_collection"
	case id != "" && ephemeral != "":
		return nil, "provide only one of collection_source_id or ephemeral_collection"
	case id != "":
		return enqueueRequestFromID(id), ""
	default:
		return enqueueRequestFromEphemeralJSON(ephemeral)
	}
}

func handleEnqueueCollectionRequest(ctx context.Context, api ControllerAPI, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	r, problem := parseEnqueueTarget(req)
	if problem != "" {
		return mcp.NewToolResultError(problem), nil
	}
	_, err := api.EnqueueCollectionRequest(ctx, connect.NewRequest(r))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{"ok": true})
}
