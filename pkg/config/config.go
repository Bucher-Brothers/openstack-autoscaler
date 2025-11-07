package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

// Config represents the configuration for the OpenStack autoscaler
type Config struct {
	Cloud CloudConfig `yaml:"cloud"`
}

// CloudConfig contains OpenStack cloud configuration
type CloudConfig struct {
	AuthURL                     string `yaml:"auth_url"`
	Username                    string `yaml:"username"`
	Password                    string `yaml:"password"`
	ProjectName                 string `yaml:"project_name"`
	ProjectID                   string `yaml:"project_id"`
	UserDomainName              string `yaml:"user_domain_name"`
	ProjectDomainName           string `yaml:"project_domain_name"`
	ApplicationCredentialID     string `yaml:"application_credential_id"`
	ApplicationCredentialName   string `yaml:"application_credential_name"`
	ApplicationCredentialSecret string `yaml:"application_credential_secret"`
	Region                      string `yaml:"region"`
	Interface                   string `yaml:"interface"`
	IdentityAPIVersion          string `yaml:"identity_api_version"`
	ComputeAPIVersion           string `yaml:"compute_api_version"`
	NetworkAPIVersion           string `yaml:"network_api_version"`
}

// NodeGroupConfig represents a configuration for a node group
type NodeGroupConfig struct {
	ID               string            `yaml:"id"`
	Name             string            `yaml:"name"`
	MinSize          int               `yaml:"minSize"`
	MaxSize          int               `yaml:"maxSize"`
	FlavorName       string            `yaml:"flavorName"`
	ImageName        string            `yaml:"imageName"`
	ImageID          string            `yaml:"imageId"`
	KeyName          string            `yaml:"keyName"`
	SecurityGroups   []string          `yaml:"securityGroups"`
	NetworkID        string            `yaml:"networkId"`
	SubnetID         string            `yaml:"subnetId"`
	FloatingIPPool   string            `yaml:"floatingIpPool"`
	AvailabilityZone string            `yaml:"availabilityZone"`
	UserData         string            `yaml:"userData"`
	UserDataFile     string            `yaml:"userDataFile"`
	Metadata         map[string]string `yaml:"metadata"`
	Labels           map[string]string `yaml:"labels"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(filepath string) (*Config, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", filepath, err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// NodeGroups are managed dynamically by the external-grpc protocol
	// No static configuration needed here

	return &config, nil
}

// LoadConfigFromEnv loads configuration from environment variables
func LoadConfigFromEnv() *CloudConfig {
	return &CloudConfig{
		AuthURL:                     getEnvOrDefault("OS_AUTH_URL", ""),
		Username:                    getEnvOrDefault("OS_USERNAME", ""),
		Password:                    getEnvOrDefault("OS_PASSWORD", ""),
		ProjectName:                 getEnvOrDefault("OS_PROJECT_NAME", ""),
		ProjectID:                   getEnvOrDefault("OS_PROJECT_ID", ""),
		UserDomainName:              getEnvOrDefault("OS_USER_DOMAIN_NAME", "Default"),
		ProjectDomainName:           getEnvOrDefault("OS_PROJECT_DOMAIN_NAME", "Default"),
		ApplicationCredentialID:     getEnvOrDefault("OS_APPLICATION_CREDENTIAL_ID", ""),
		ApplicationCredentialName:   getEnvOrDefault("OS_APPLICATION_CREDENTIAL_NAME", ""),
		ApplicationCredentialSecret: getEnvOrDefault("OS_APPLICATION_CREDENTIAL_SECRET", ""),
		Region:                      getEnvOrDefault("OS_REGION_NAME", ""),
		Interface:                   getEnvOrDefault("OS_INTERFACE", "public"),
		IdentityAPIVersion:          getEnvOrDefault("OS_IDENTITY_API_VERSION", "3"),
		ComputeAPIVersion:           getEnvOrDefault("OS_COMPUTE_API_VERSION", "2.1"),
		NetworkAPIVersion:           getEnvOrDefault("OS_NETWORK_API_VERSION", "2.0"),
	}
}

// ValidateAuth validates that either application credentials or username/password are provided
func (c *CloudConfig) ValidateAuth() error {
	hasAppCredID := c.ApplicationCredentialID != ""
	hasAppCredSecret := c.ApplicationCredentialSecret != ""
	hasAppCredName := c.ApplicationCredentialName != ""
	hasUsername := c.Username != ""
	hasPassword := c.Password != ""

	// Check if application credentials are complete
	appCredIDAuth := hasAppCredID && hasAppCredSecret
	appCredNameAuth := hasAppCredName && hasAppCredSecret && hasUsername

	// Check if username/password auth is complete
	usernamePasswordAuth := hasUsername && hasPassword

	// Must have either complete application credentials or username/password
	if !appCredIDAuth && !appCredNameAuth && !usernamePasswordAuth {
		return fmt.Errorf("authentication configuration incomplete: must provide either " +
			"(OS_APPLICATION_CREDENTIAL_ID + OS_APPLICATION_CREDENTIAL_SECRET) or " +
			"(OS_APPLICATION_CREDENTIAL_NAME + OS_APPLICATION_CREDENTIAL_SECRET + OS_USERNAME) or " +
			"(OS_USERNAME + OS_PASSWORD)")
	}

	// Don't allow mixing application credentials with password auth
	if (appCredIDAuth || appCredNameAuth) && hasPassword {
		return fmt.Errorf("cannot mix application credentials with password authentication")
	}

	return nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
