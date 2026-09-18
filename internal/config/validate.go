package config

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

func (c *Config) Validate(knownResources []string) error {
	var errs []error

	if len(c.Accounts) == 0 {
		errs = append(errs, errors.New("accounts: at least one account is required"))
	}

	if c.Storage.Backend != "elasticsearch" {
		errs = append(errs, fmt.Errorf("storage.backend: %q is not a supported backend", c.Storage.Backend))
	} else if len(c.Storage.Elasticsearch.Addresses) == 0 {
		errs = append(errs, errors.New("storage.elasticsearch.addresses: at least one address is required"))
	}

	if c.Storage.Elasticsearch.BatchSize < 1 {
		errs = append(errs, errors.New("storage.elasticsearch.batch_size: must be greater than zero"))
	}

	seen := make(map[string]struct{}, len(c.Accounts))
	for i, a := range c.Accounts {
		where := fmt.Sprintf("accounts[%d]", i)
		if a.Name == "" {
			errs = append(errs, fmt.Errorf("%s.name: is required", where))
		} else {
			where = "accounts." + a.Name
			if _, dup := seen[a.Name]; dup {
				errs = append(errs, fmt.Errorf("%s: duplicate account name", where))
			}
			seen[a.Name] = struct{}{}
		}
		if a.ID == "" {
			errs = append(errs, fmt.Errorf("%s.id: is required", where))
		}
		if a.RoleARN == "" {
			errs = append(errs, fmt.Errorf("%s.role_arn: is required", where))
		}
		if a.ScrapeInterval <= 0 {
			errs = append(errs, fmt.Errorf("%s.scrape_interval: must be greater than zero", where))
		}
		if len(a.Resources) == 0 {
			errs = append(errs, fmt.Errorf("%s.resources: is empty and no global resources are configured", where))
		}
		for _, r := range a.Resources {
			if !slices.Contains(knownResources, r) {
				errs = append(errs, fmt.Errorf("%s.resources: unknown resource type %q, known types are %s",
					where, r, strings.Join(knownResources, ", ")))
			}
		}
	}

	return errors.Join(errs...)
}
