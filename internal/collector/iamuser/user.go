package iamuser

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/windkube/aws-metrics-exporter/internal/collector"
)

const ResourceType = "aws_iam_user"

// API is the subset of the IAM client this collector uses.
type API interface {
	ListUsers(context.Context, *iam.ListUsersInput, ...func(*iam.Options)) (*iam.ListUsersOutput, error)
	ListUserTags(context.Context, *iam.ListUserTagsInput, ...func(*iam.Options)) (*iam.ListUserTagsOutput, error)
	ListAccessKeys(context.Context, *iam.ListAccessKeysInput, ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error)
	ListMFADevices(context.Context, *iam.ListMFADevicesInput, ...func(*iam.Options)) (*iam.ListMFADevicesOutput, error)
	ListAttachedUserPolicies(context.Context, *iam.ListAttachedUserPoliciesInput, ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error)
	ListUserPolicies(context.Context, *iam.ListUserPoliciesInput, ...func(*iam.Options)) (*iam.ListUserPoliciesOutput, error)
	ListGroupsForUser(context.Context, *iam.ListGroupsForUserInput, ...func(*iam.Options)) (*iam.ListGroupsForUserOutput, error)
}

// User is the AWS representation of an IAM user with the related objects that only show up in
// separate calls.
type User struct {
	types.User
	AccessKeys       []types.AccessKeyMetadata `json:"AccessKeys"`
	MFADevices       []types.MFADevice         `json:"MFADevices"`
	AttachedPolicies []types.AttachedPolicy    `json:"AttachedPolicies"`
	InlinePolicies   []string                  `json:"InlinePolicies"`
	Groups           []types.Group             `json:"Groups"`
}

type Collector struct {
	newAPI func(aws.Config) API
}

func New() *Collector {
	return &Collector{newAPI: func(cfg aws.Config) API { return iam.NewFromConfig(cfg) }}
}

func (c *Collector) Type() string           { return ResourceType }
func (c *Collector) Scope() collector.Scope { return collector.ScopeGlobal }

func (c *Collector) Collect(ctx context.Context, cfg aws.Config, emit collector.EmitFunc) error {
	api := c.newAPI(cfg)

	pages := iam.NewListUsersPaginator(api, &iam.ListUsersInput{})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("listing iam users: %w", err)
		}

		for _, u := range page.Users {
			user, err := describe(ctx, api, u)
			if err != nil {
				return err
			}

			if err := emit(collector.Resource{
				ID:   aws.ToString(u.Arn),
				Name: aws.ToString(u.UserName),
				Data: user,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func describe(ctx context.Context, api API, u types.User) (User, error) {
	name := u.UserName
	user := User{User: u}

	tags, err := api.ListUserTags(ctx, &iam.ListUserTagsInput{UserName: name})
	if err != nil {
		return User{}, fmt.Errorf("listing tags of iam user %s: %w", aws.ToString(name), err)
	}
	user.Tags = tags.Tags

	keys, err := api.ListAccessKeys(ctx, &iam.ListAccessKeysInput{UserName: name})
	if err != nil {
		return User{}, fmt.Errorf("listing access keys of iam user %s: %w", aws.ToString(name), err)
	}
	user.AccessKeys = keys.AccessKeyMetadata

	mfa, err := api.ListMFADevices(ctx, &iam.ListMFADevicesInput{UserName: name})
	if err != nil {
		return User{}, fmt.Errorf("listing mfa devices of iam user %s: %w", aws.ToString(name), err)
	}
	user.MFADevices = mfa.MFADevices

	attached, err := api.ListAttachedUserPolicies(ctx, &iam.ListAttachedUserPoliciesInput{UserName: name})
	if err != nil {
		return User{}, fmt.Errorf("listing attached policies of iam user %s: %w", aws.ToString(name), err)
	}
	user.AttachedPolicies = attached.AttachedPolicies

	inline, err := api.ListUserPolicies(ctx, &iam.ListUserPoliciesInput{UserName: name})
	if err != nil {
		return User{}, fmt.Errorf("listing inline policies of iam user %s: %w", aws.ToString(name), err)
	}
	user.InlinePolicies = inline.PolicyNames

	groups, err := api.ListGroupsForUser(ctx, &iam.ListGroupsForUserInput{UserName: name})
	if err != nil {
		return User{}, fmt.Errorf("listing groups of iam user %s: %w", aws.ToString(name), err)
	}
	user.Groups = groups.Groups

	return user, nil
}
