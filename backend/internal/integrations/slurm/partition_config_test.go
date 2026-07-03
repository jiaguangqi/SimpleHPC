package slurm

import (
	"strings"
	"testing"
)

func TestPartitionConfigRoundTrip(t *testing.T) {
	line := "PartitionName=debug AllowGroups=ALL AllowAccounts=ALL AllowQos=normal Default=YES QoS=normal MaxTime=UNLIMITED MaxCPUsPerNode=20 Nodes=cae State=UP TotalCPUs=20 TotalNodes=1"
	values := parseKeyValueLine(line)
	item := PartitionConfig{
		Name: values["PartitionName"], Nodes: values["Nodes"], MaxTime: values["MaxTime"],
		MaxCPUsPerNode: values["MaxCPUsPerNode"], AllowGroups: values["AllowGroups"],
		AllowAccounts: values["AllowAccounts"], AllowQOS: values["AllowQos"],
		Default: values["Default"], State: values["State"], QOS: values["QoS"],
	}
	rendered := renderPartitionConfig(item)
	for _, expected := range []string{"PartitionName=debug", "Nodes=cae", "Default=YES", "State=UP", "QOS=normal"} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("rendered config missing %q: %s", expected, rendered)
		}
	}
}

func TestValidatePartitionConfigRejectsShellCharacters(t *testing.T) {
	item := PartitionConfig{Name: "debug", Nodes: "cae;touch/tmp/x", Default: "NO", State: "UP"}
	if err := validatePartitionConfig(item); err == nil {
		t.Fatal("expected invalid node expression to be rejected")
	}
}

func TestMergePartitionConfigPreservesUnknownFields(t *testing.T) {
	line := "PartitionName=debug Nodes=cae Default=YES MaxTime=INFINITE State=UP OverSubscribe=YES:20"
	item := PartitionConfig{
		Name: "debug", Nodes: "cae", Default: "YES", State: "UP",
		MaxTime: "UNLIMITED", MaxCPUsPerNode: "UNLIMITED",
		AllowGroups: "ALL", AllowAccounts: "ALL", AllowQOS: "ALL",
	}
	rendered := mergePartitionConfigLine(line, item)
	if !strings.Contains(rendered, "OverSubscribe=YES:20") {
		t.Fatalf("unknown field was removed: %s", rendered)
	}
	if !strings.Contains(rendered, "MaxTime=UNLIMITED") {
		t.Fatalf("edited field was not updated: %s", rendered)
	}
}
