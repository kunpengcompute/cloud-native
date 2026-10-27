package templatecreator

import "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/publisher"

// ManifestSchemaVersion 是当前 completion manifest 的 schema 版本。
const ManifestSchemaVersion = publisher.ManifestSchemaVersion

// ArtifactManifest 是全部 Template 对象写入成功后最后发布的 JSON completion manifest。
// 它是公开别名，调用方无需导入 internal publisher package 即可解析最终产物清单。
type ArtifactManifest = publisher.Manifest

// ArtifactObject 描述 ArtifactManifest 引用的一个对象及其大小和 SHA-256。
type ArtifactObject = publisher.ObjectRecord
