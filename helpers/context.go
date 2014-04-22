package helpers

import (
	"encoding/json"
	"fmt"
	"time"

	ginkgoconfig "github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf-experimental/cf-test-helpers/cf"
	"github.com/vito/cmdtest"
	. "github.com/vito/cmdtest/matchers"
)

type ConfiguredContext struct {
	config Config

	organizationName string
	spaceName        string

	quotaDefinitionName string
	quotaDefinitionGUID string

	regularUserUsername string
	regularUserPassword string

	isPersistent bool
}

type quotaDefinition struct {
	Name string `json:"name"`

	NonBasicServicesAllowed bool `json:"non_basic_services_allowed"`

	TotalServices int `json:"total_services"`
	TotalRoutes   int `json:"total_routes"`

	MemoryLimit int `json:"memory_limit"`
}

func NewContext(config Config) *ConfiguredContext {
	node := ginkgoconfig.GinkgoConfig.ParallelNode
	timeTag := time.Now().Format("2006_01_02-15h04m05.999s")

	return &ConfiguredContext{
		config: config,

		quotaDefinitionName: fmt.Sprintf("cats-quota-%d-%s", node, timeTag),

		organizationName: fmt.Sprintf("cats-org-%d-%s", node, timeTag),
		spaceName:        fmt.Sprintf("cats-space-%d-%s", node, timeTag),

		regularUserUsername: fmt.Sprintf("cats-user-%d-%s", node, timeTag),
		regularUserPassword: "meow",

		isPersistent: false,
	}
}

func NewPersistentAppContext(config Config) *ConfiguredContext {
	baseContext := NewContext(config)

	baseContext.quotaDefinitionName = config.PersistentAppQuotaName
	baseContext.organizationName = config.PersistentAppOrg
	baseContext.spaceName = config.PersistentAppSpace
	baseContext.isPersistent = true

	return baseContext
}

func (context *ConfiguredContext) Setup() {
	cf.AsUser(context.AdminUserContext(), func() {
		definition := quotaDefinition{
			Name: context.quotaDefinitionName,

			TotalServices: 100,
			TotalRoutes:   1000,

			MemoryLimit: 10240,

			NonBasicServicesAllowed: true,
		}

		definitionPayload, err := json.Marshal(definition)
		Expect(err).ToNot(HaveOccurred())

		var response cf.GenericResource

		cf.ApiRequest("POST", "/v2/quota_definitions", &response, string(definitionPayload))

		context.quotaDefinitionGUID = response.Metadata.Guid
		fmt.Printf("QuotaDefinition Response: %#v\n", response)
		println("GUID", context.quotaDefinitionGUID)

		Expect(cf.Cf("create-user", context.regularUserUsername, context.regularUserPassword)).To(SayBranches(
			cmdtest.ExpectBranch{"OK", func() {}},
			cmdtest.ExpectBranch{"scim_resource_already_exists", func() {}},
		))

		Expect(cf.Cf("create-org", context.organizationName)).To(ExitWith(0))
		Expect(cf.Cf("set-quota", context.organizationName, definition.Name)).To(ExitWith(0))
	})
}

func (context *ConfiguredContext) Teardown() {
	cf.AsUser(context.AdminUserContext(), func() {
		Expect(cf.Cf("delete-user", "-f", context.regularUserUsername)).To(Say("OK"))

		if !context.isPersistent {
			Expect(cf.Cf("delete-org", "-f", context.organizationName)).To(Say("OK"))

			cf.ApiRequest(
				"DELETE",
				"/v2/quota_definitions/"+context.quotaDefinitionGUID+"?recursive=true",
				nil,
			)
		}
	})
}

func (context *ConfiguredContext) AdminUserContext() cf.UserContext {
	return cf.NewUserContext(
		context.config.ApiEndpoint,
		context.config.AdminUser,
		context.config.AdminPassword,
		"",
		"",
		context.config.SkipSSLValidation,
	)
}

func (context *ConfiguredContext) RegularUserContext() cf.UserContext {
	return cf.NewUserContext(
		context.config.ApiEndpoint,
		context.regularUserUsername,
		context.regularUserPassword,
		context.organizationName,
		context.spaceName,
		context.config.SkipSSLValidation,
	)
}
