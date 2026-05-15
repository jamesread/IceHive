package main

import (
	"context"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	icehivev1 "github.com/icehive/icehive/services/common/pkg/gen/icehive/v1"
	"github.com/icehive/icehive/services/common/pkg/gen/icehive/v1/icehivev1connect"
)

const collectorJmapType = "collector-jmap"

func controllerClient(base string) icehivev1connect.ControllerServiceClient {
	return icehivev1connect.NewControllerServiceClient(http.DefaultClient, strings.TrimRight(strings.TrimSpace(base), "/"))
}

func listJmapCollectionSources(ctx context.Context, base string) ([]*icehivev1.CollectionSource, error) {
	cli := controllerClient(base)
	res, err := cli.ListCollectionSources(ctx, connect.NewRequest(&icehivev1.ListCollectionSourcesRequest{
		CollectorType: collectorJmapType,
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetSources(), nil
}

func reportCollectionSourceRun(
	ctx context.Context,
	base string,
	sourceID string,
	success bool,
	errMsg string,
	nextDueUnixMs int64,
) error {
	if strings.TrimSpace(sourceID) == "" {
		return nil
	}
	cli := controllerClient(base)
	_, err := cli.ReportCollectionSourceRun(ctx, connect.NewRequest(&icehivev1.ReportCollectionSourceRunRequest{
		Id:            sourceID,
		RunUnixMs:     time.Now().UTC().UnixMilli(),
		Success:       success,
		Error:         errMsg,
		NextDueUnixMs: nextDueUnixMs,
	}))
	return err
}
