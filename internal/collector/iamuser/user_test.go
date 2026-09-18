package iamuser

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windkube/aws-metrics-exporter/internal/collector"
)

type fakeIAM struct {
	pages     [][]types.User
	nextPage  int
	tagsErr   error
	callCount map[string]int
}

func (f *fakeIAM) count(name string) {
	if f.callCount == nil {
		f.callCount = map[string]int{}
	}
	f.callCount[name]++
}

func (f *fakeIAM) ListUsers(context.Context, *iam.ListUsersInput, ...func(*iam.Options)) (*iam.ListUsersOutput, error) {
	f.count("ListUsers")

	page := f.pages[f.nextPage]
	f.nextPage++
	truncated := f.nextPage < len(f.pages)

	out := &iam.ListUsersOutput{Users: page, IsTruncated: truncated}
	if truncated {
		out.Marker = aws.String("next")
	}

	return out, nil
}

func (f *fakeIAM) ListUserTags(context.Context, *iam.ListUserTagsInput, ...func(*iam.Options)) (*iam.ListUserTagsOutput, error) {
	f.count("ListUserTags")
	if f.tagsErr != nil {
		return nil, f.tagsErr
	}
	return &iam.ListUserTagsOutput{Tags: []types.Tag{{Key: aws.String("team"), Value: aws.String("platform")}}}, nil
}

func (f *fakeIAM) ListAccessKeys(context.Context, *iam.ListAccessKeysInput, ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error) {
	f.count("ListAccessKeys")
	return &iam.ListAccessKeysOutput{AccessKeyMetadata: []types.AccessKeyMetadata{{AccessKeyId: aws.String("AKIA1")}}}, nil
}

func (f *fakeIAM) ListMFADevices(context.Context, *iam.ListMFADevicesInput, ...func(*iam.Options)) (*iam.ListMFADevicesOutput, error) {
	f.count("ListMFADevices")
	return &iam.ListMFADevicesOutput{MFADevices: []types.MFADevice{{SerialNumber: aws.String("arn:mfa")}}}, nil
}

func (f *fakeIAM) ListAttachedUserPolicies(context.Context, *iam.ListAttachedUserPoliciesInput, ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error) {
	f.count("ListAttachedUserPolicies")
	return &iam.ListAttachedUserPoliciesOutput{AttachedPolicies: []types.AttachedPolicy{{PolicyName: aws.String("ReadOnly")}}}, nil
}

func (f *fakeIAM) ListUserPolicies(context.Context, *iam.ListUserPoliciesInput, ...func(*iam.Options)) (*iam.ListUserPoliciesOutput, error) {
	f.count("ListUserPolicies")
	return &iam.ListUserPoliciesOutput{PolicyNames: []string{"inline-one"}}, nil
}

func (f *fakeIAM) ListGroupsForUser(context.Context, *iam.ListGroupsForUserInput, ...func(*iam.Options)) (*iam.ListGroupsForUserOutput, error) {
	f.count("ListGroupsForUser")
	return &iam.ListGroupsForUserOutput{Groups: []types.Group{{GroupName: aws.String("admins")}}}, nil
}

func collectorWith(api API) *Collector {
	return &Collector{newAPI: func(aws.Config) API { return api }}
}

func user(name string) types.User {
	return types.User{
		UserName: aws.String(name),
		Arn:      aws.String("arn:aws:iam::111111111111:user/" + name),
	}
}

func TestCollectWalksEveryPageAndEnrichesEachUser(t *testing.T) {
	api := &fakeIAM{pages: [][]types.User{{user("kw"), user("bot")}, {user("ci")}}}

	var got []collector.Resource
	require.NoError(t, collectorWith(api).Collect(t.Context(), aws.Config{}, func(r collector.Resource) error {
		got = append(got, r)
		return nil
	}))

	require.Len(t, got, 3)
	assert.Equal(t, "kw", got[0].Name)
	assert.Equal(t, "arn:aws:iam::111111111111:user/kw", got[0].ID)
	assert.Equal(t, 2, api.callCount["ListUsers"])
	assert.Equal(t, 3, api.callCount["ListAccessKeys"])

	encoded, err := json.Marshal(got[0].Data)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(encoded, &decoded))

	// The embedded AWS type is promoted, so the document is the AWS JSON shape plus the
	// enrichment the list call does not return.
	assert.Equal(t, "kw", decoded["UserName"])
	assert.Len(t, decoded["AccessKeys"], 1)
	assert.Len(t, decoded["MFADevices"], 1)
	assert.Len(t, decoded["Groups"], 1)
	assert.Equal(t, []any{"inline-one"}, decoded["InlinePolicies"])
	assert.Len(t, decoded["Tags"], 1)
}

func TestCollectStopsOnEnrichmentFailure(t *testing.T) {
	api := &fakeIAM{pages: [][]types.User{{user("kw")}}, tagsErr: errors.New("throttled")}

	err := collectorWith(api).Collect(t.Context(), aws.Config{}, func(collector.Resource) error {
		t.Fatal("no resource should be emitted")
		return nil
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing tags of iam user kw")
}

func TestCollectPropagatesEmitFailure(t *testing.T) {
	api := &fakeIAM{pages: [][]types.User{{user("kw"), user("bot")}}}
	sentinel := errors.New("backend down")

	emitted := 0
	err := collectorWith(api).Collect(t.Context(), aws.Config{}, func(collector.Resource) error {
		emitted++
		return sentinel
	})

	require.ErrorIs(t, err, sentinel)
	assert.Equal(t, 1, emitted)
}

func TestScopeIsGlobal(t *testing.T) {
	assert.Equal(t, collector.ScopeGlobal, New().Scope())
	assert.Equal(t, ResourceType, New().Type())
}
