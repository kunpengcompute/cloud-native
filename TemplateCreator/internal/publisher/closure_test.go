package publisher

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"

	upstreamstorage "gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/storage"
	upstreamheader "gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/storage/header"
)

func TestResolveClosureCollectsInfra202609FinalArtifactsAndAncestors(t *testing.T) {
	root := t.TempDir()
	finalID := uuid.New()
	ancestorID := uuid.New()
	for _, name := range []string{upstreamstorage.MemfileName, upstreamstorage.RootfsName} {
		writeClosureHeader(t, root, finalID, name, []uuid.UUID{ancestorID, finalID})
		writeClosureDataObject(t, root, finalID, name)
		writeClosureDataObject(t, root, ancestorID, name)
	}
	finalFiles := upstreamstorage.TemplateFiles{BuildID: finalID.String()}
	writeClosureObject(t, root, finalFiles.StorageSnapfilePath())
	writeClosureObject(t, root, finalFiles.StorageMetadataPath())

	closure, err := ResolveClosure(root, finalID.String())
	if err != nil {
		t.Fatalf("ResolveClosure() error = %v", err)
	}
	if !reflect.DeepEqual(closure.AncestorBuildIDs, []string{ancestorID.String()}) {
		t.Fatalf("AncestorBuildIDs = %v", closure.AncestorBuildIDs)
	}
	if len(closure.Objects) != 8 {
		t.Fatalf("len(Objects) = %d, want 8", len(closure.Objects))
	}
	for index := 1; index < len(closure.Objects); index++ {
		if closure.Objects[index-1].Key >= closure.Objects[index].Key {
			t.Fatalf("objects not sorted: %#v", closure.Objects)
		}
	}
}

func TestResolveAllKeepsEveryStagingObject(t *testing.T) {
	root := t.TempDir()
	finalID := uuid.New()
	ancestorID := uuid.New()
	for _, key := range []string{
		finalID.String() + "/memfile",
		finalID.String() + "/memfile.header",
		ancestorID.String() + "/rootfs.ext4",
		ancestorID.String() + "/metadata.json",
	} {
		writeClosureObject(t, root, key)
	}

	closure, err := ResolveAll(root, finalID.String())
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}
	if len(closure.Objects) != 4 {
		t.Fatalf("len(Objects) = %d, want 4", len(closure.Objects))
	}
	if !reflect.DeepEqual(closure.AncestorBuildIDs, []string{ancestorID.String()}) {
		t.Fatalf("AncestorBuildIDs = %v, want %v", closure.AncestorBuildIDs, []string{ancestorID.String()})
	}
	for _, object := range closure.Objects {
		if !strings.HasPrefix(object.Key, finalID.String()+"/") && !strings.HasPrefix(object.Key, ancestorID.String()+"/") {
			t.Fatalf("ResolveAll changed object key: %q", object.Key)
		}
	}
}

func TestResolveClosureDeduplicatesAncestorReferencedByBothHeaders(t *testing.T) {
	root := t.TempDir()
	finalID := uuid.New()
	ancestorID := uuid.New()
	for _, name := range []string{upstreamstorage.MemfileName, upstreamstorage.RootfsName} {
		writeClosureHeader(t, root, finalID, name, []uuid.UUID{ancestorID, finalID})
		writeClosureDataObject(t, root, finalID, name)
		writeClosureDataObject(t, root, ancestorID, name)
	}
	finalFiles := upstreamstorage.TemplateFiles{BuildID: finalID.String()}
	writeClosureObject(t, root, finalFiles.StorageSnapfilePath())
	writeClosureObject(t, root, finalFiles.StorageMetadataPath())

	closure, err := ResolveClosure(root, finalID.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(closure.AncestorBuildIDs) != 1 {
		t.Fatalf("ancestors = %v, want one", closure.AncestorBuildIDs)
	}
}

func TestResolveClosureSkipsEmptyBlockBuildID(t *testing.T) {
	root := t.TempDir()
	finalID := uuid.New()
	for _, name := range []string{upstreamstorage.MemfileName, upstreamstorage.RootfsName} {
		writeClosureHeader(t, root, finalID, name, []uuid.UUID{uuid.Nil, finalID})
		writeClosureDataObject(t, root, finalID, name)
	}
	finalFiles := upstreamstorage.TemplateFiles{BuildID: finalID.String()}
	writeClosureObject(t, root, finalFiles.StorageSnapfilePath())
	writeClosureObject(t, root, finalFiles.StorageMetadataPath())

	closure, err := ResolveClosure(root, finalID.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(closure.Objects) != 6 || len(closure.AncestorBuildIDs) != 0 {
		t.Fatalf("closure = %#v, want six final objects and no ancestors", closure)
	}
}

func TestResolveClosureRejectsMissingReferencedObject(t *testing.T) {
	root := t.TempDir()
	finalID := uuid.New()
	writeClosureHeader(t, root, finalID, upstreamstorage.MemfileName, []uuid.UUID{finalID})
	writeClosureHeader(t, root, finalID, upstreamstorage.RootfsName, []uuid.UUID{finalID})

	_, err := ResolveClosure(root, finalID.String())
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("ResolveClosure() error = %v, want missing object error", err)
	}
}

func TestResolveClosureRejectsUnsafeFinalBuildID(t *testing.T) {
	_, err := ResolveClosure(t.TempDir(), "../outside")
	if err == nil || !strings.Contains(err.Error(), "BuildID") {
		t.Fatalf("ResolveClosure() error = %v, want BuildID error", err)
	}
}

func TestResolveClosureRejectsInvalidHeaderMapping(t *testing.T) {
	root := t.TempDir()
	finalID := uuid.New()
	mappings := []*upstreamheader.BuildMap{{Offset: 0, Length: 4096, BuildId: finalID}}
	metadata := &upstreamheader.Metadata{Version: 3, BlockSize: 4096, Size: 8192, BuildId: finalID, BaseBuildId: finalID}
	writeSerializedClosureHeader(t, root, finalID, upstreamstorage.MemfileName, metadata, mappings)

	_, err := ResolveClosure(root, finalID.String())
	if err == nil || !strings.Contains(err.Error(), "validate header") {
		t.Fatalf("ResolveClosure() error = %v, want header validation error", err)
	}
}

func writeClosureHeader(t *testing.T, root string, finalID uuid.UUID, name string, builds []uuid.UUID) {
	t.Helper()
	mappings := make([]*upstreamheader.BuildMap, len(builds))
	for index, id := range builds {
		mappings[index] = &upstreamheader.BuildMap{Offset: uint64(index * 4096), Length: 4096, BuildId: id}
	}
	metadata := &upstreamheader.Metadata{
		Version: 3, BlockSize: 4096, Size: uint64(len(builds) * 4096),
		BuildId: finalID, BaseBuildId: finalID,
	}
	writeSerializedClosureHeader(t, root, finalID, name, metadata, mappings)
}

func writeSerializedClosureHeader(t *testing.T, root string, finalID uuid.UUID, name string, metadata *upstreamheader.Metadata, mappings []*upstreamheader.BuildMap) {
	t.Helper()
	data, err := upstreamheader.Serialize(metadata, mappings)
	if err != nil {
		t.Fatal(err)
	}
	files := upstreamstorage.TemplateFiles{BuildID: finalID.String()}
	key := files.StorageMemfileHeaderPath()
	if name == upstreamstorage.RootfsName {
		key = files.StorageRootfsHeaderPath()
	}
	writeClosureContent(t, root, key, data)
}

func writeClosureDataObject(t *testing.T, root string, buildID uuid.UUID, name string) {
	t.Helper()
	files := upstreamstorage.TemplateFiles{BuildID: buildID.String()}
	key := files.StorageMemfilePath()
	if name == upstreamstorage.RootfsName {
		key = files.StorageRootfsPath()
	}
	writeClosureObject(t, root, key)
}

func writeClosureObject(t *testing.T, root, key string) {
	t.Helper()
	writeClosureContent(t, root, key, []byte("object:"+key))
}

func writeClosureContent(t *testing.T, root, key string, content []byte) {
	t.Helper()
	objectPath := filepath.Join(root, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(objectPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(objectPath, content, 0o644); err != nil {
		t.Fatal(err)
	}
}
