package config

import (
	"os"
	"strings"
)

type Config struct {
	Addr                  string
	PublicURL             string
	FrontendDir           string
	DatabaseURL           string
	RedisURL              string
	LDAPURL               string
	LDAPBaseDN            string
	LDAPAdminDN           string
	LDAPAdminPassword     string
	SlurmBinDir           string
	SlurmConfigPath       string
	SlurmDefaultAccount   string
	SlurmDefaultPartition string
	StorageRoots          []string
	RBACMode              string
}

func Load() Config {
	return Config{
		Addr:                  env("SIMPLEHPC_ADDR", ":8080"),
		PublicURL:             strings.TrimRight(env("SIMPLEHPC_PUBLIC_URL", "http://127.0.0.1:8080"), "/"),
		FrontendDir:           env("SIMPLEHPC_FRONTEND_DIR", ".."),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		RedisURL:              os.Getenv("REDIS_URL"),
		LDAPURL:               env("LDAP_URL", "ldap://127.0.0.1:389"),
		LDAPBaseDN:            env("LDAP_BASE_DN", "dc=simplehpc,dc=local"),
		LDAPAdminDN:           env("LDAP_ADMIN_DN", "cn=admin,dc=simplehpc,dc=local"),
		LDAPAdminPassword:     os.Getenv("LDAP_ADMIN_PASSWORD"),
		SlurmBinDir:           env("SLURM_BIN_DIR", "/opt/slurm/current/bin"),
		SlurmConfigPath:       env("SLURM_CONFIG_PATH", "/etc/slurm/slurm.conf"),
		SlurmDefaultAccount:   env("SLURM_DEFAULT_ACCOUNT", "simplehpc"),
		SlurmDefaultPartition: env("SLURM_DEFAULT_PARTITION", "debug"),
		StorageRoots:          splitCSV(env("STORAGE_ROOTS", "/data/home,/data/share,/data/recycle,/data/scratch")),
		RBACMode:              env("RBAC_MODE", "legacy"),
	}
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
