package templatecreator

import (
	"strings"
	"testing"
)

func TestRenderRegistrationSQLUsesTaskNumberAndLocalDefaults(t *testing.T) {
	rendered, err := renderRegistrationSQL(registrationSQLRequest{
		StoreURL:       "file:///store",
		TemplateID:     "template-1",
		BuildID:        "11111111-1111-1111-1111-111111111111",
		ImageRef:       "registry.example/debian's:latest",
		IdempotencyKey: "task-demo-v1",
		Metadata: BuildMetadata{
			TemplateVersion:    "v2",
			KernelVersion:      "vmlinux-6.1",
			FirecrackerVersion: "v1.14.1",
			EnvdVersion:        "0.6.10",
			RootfsSizeMB:       4096,
			Resources: BuildSpec{
				VCPU: 2, MemoryMB: 1024, DiskSizeMB: 4096,
				StartCommand: "printf 'ready'", ReadyCommand: "curl localhost",
			},
			MachineInfo: MachineInfo{
				CPUArchitecture: "amd64", CPUFamily: "6", CPUModel: "85",
				CPUModelName: "Example CPU", CPUFlags: []string{"sse4_2", "avx2"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	result := rendered.Result

	if result.ObjectKey != "11111111-1111-1111-1111-111111111111/registration.sql" {
		t.Fatalf("ObjectKey = %q", result.ObjectKey)
	}
	if result.LocalPath != "/store/11111111-1111-1111-1111-111111111111/registration.sql" {
		t.Fatalf("LocalPath = %q", result.LocalPath)
	}
	for _, want := range []string{
		`\set team_id 'REPLACE_WITH_TEAM_ID'`,
		`\set namespace 'local-dev-team'`,
		`\set cluster_id ''`,
		`\set cluster_node_id 'local'`,
		`\set template_alias 'task-demo-v1'`,
		`registry.example/debian''s:latest`,
		`printf ''ready''`,
		`["sse4_2","avx2"]`,
		`INSERT INTO envs`,
		`team_id, cluster_id, source`,
		`NULLIF(:'cluster_id', '')::uuid, 'template'`,
		`INSERT INTO env_builds`,
		`INSERT INTO env_build_assignments`,
		`INSERT INTO env_aliases`,
		`Redis cache`,
	} {
		if !strings.Contains(rendered.Contents, want) {
			t.Fatalf("SQL missing %q:\n%s", want, rendered.Contents)
		}
	}
	if len(result.Warnings) < 3 || !strings.Contains(strings.Join(result.Warnings, "\n"), "team_id") {
		t.Fatalf("Warnings = %#v", result.Warnings)
	}
	if !strings.Contains(strings.Join(result.Warnings, "\n"), "cluster_id") {
		t.Fatalf("Warnings missing cluster_id guidance: %#v", result.Warnings)
	}
	if strings.Contains(rendered.Contents, "00000000-0000-0000-0000-000000000000") {
		t.Fatalf("local registration SQL must store cluster_id as NULL, not nil UUID:\n%s", rendered.Contents)
	}
	if !strings.Contains(strings.Join(result.Warnings, "\n"), "NULL") {
		t.Fatalf("Warnings missing local NULL cluster guidance: %#v", result.Warnings)
	}
}

func TestRenderRegistrationSQLUsesAliasPlaceholderWithoutTaskNumber(t *testing.T) {
	rendered, err := renderRegistrationSQL(registrationSQLRequest{
		StoreURL:   "file:///store",
		TemplateID: "template-1",
		BuildID:    "11111111-1111-1111-1111-111111111111",
		ImageRef:   "debian:bookworm-slim",
		Metadata: BuildMetadata{
			TemplateVersion: "v2", KernelVersion: "kernel", FirecrackerVersion: "firecracker", EnvdVersion: "envd",
			RootfsSizeMB: 4096,
			Resources:    BuildSpec{VCPU: 2, MemoryMB: 1024, DiskSizeMB: 4096},
			MachineInfo:  MachineInfo{CPUArchitecture: "amd64"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	result := rendered.Result
	if !strings.Contains(rendered.Contents, `\set template_alias 'REPLACE_WITH_TEMPLATE_ALIAS'`) {
		t.Fatalf("SQL does not contain alias placeholder:\n%s", rendered.Contents)
	}
	if !strings.Contains(rendered.Contents, `jsonb_array_elements_text('[]'::jsonb)`) {
		t.Fatalf("SQL must encode missing CPU flags as an empty JSON array:\n%s", rendered.Contents)
	}
	if !strings.Contains(strings.Join(result.Warnings, "\n"), "任务编号为空") {
		t.Fatalf("Warnings = %#v", result.Warnings)
	}
}
