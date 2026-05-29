package main

import (
	"context"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/icehive/icehive/services/common/pkg/controllerurl"
	icehivev1 "github.com/icehive/icehive/services/common/pkg/gen/icehive/v1"
	"github.com/icehive/icehive/services/common/pkg/gen/icehive/v1/icehivev1connect"
)

const (
	collectorGitHubType  = "collector-github"
	configKeyGitHubToken = "github.token"
)

func controllerClient(base string) icehivev1connect.ControllerServiceClient {
	return icehivev1connect.NewControllerServiceClient(controllerurl.HTTPClient(), strings.TrimRight(strings.TrimSpace(base), "/"))
}

func listGithubCollectionSources(ctx context.Context, base string) ([]*icehivev1.CollectionSource, error) {
	cli := controllerClient(base)
	res, err := cli.ListCollectionSources(ctx, connect.NewRequest(&icehivev1.ListCollectionSourcesRequest{
		CollectorType: collectorGitHubType,
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetSources(), nil
}

func getGitHubToken(ctx context.Context, base string) (string, error) {
	cli := controllerClient(base)
	res, err := cli.GetConfig(ctx, connect.NewRequest(&icehivev1.GetConfigRequest{Key: configKeyGitHubToken}))
	if err != nil {
		return "", err
	}
	v := res.Msg.GetVar()
	if v == nil {
		return "", nil
	}
	return strings.TrimSpace(v.GetValue()), nil
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
