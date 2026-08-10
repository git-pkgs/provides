package artifacts

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net/mail"
	"strings"

	"github.com/git-pkgs/archives"
	"github.com/git-pkgs/provides"
)

const (
	jarEntriesSource       = "jar entries"
	classFileMagic         = 0xcafebabe
	classVersionSize       = 4
	classIdentitySize      = 6
	classMemberHeaderSize  = 6
	classAttributeNameSize = 2
	classIndexSize         = 2
	constantFourByteSize   = 4
	constantEightByteSize  = 8
	constantMethodSize     = 3
	constantUTF8           = 1
	constantInteger        = 3
	constantFloat          = 4
	constantLong           = 5
	constantDouble         = 6
	constantClass          = 7
	constantString         = 8
	constantFieldRef       = 9
	constantMethodRef      = 10
	constantInterfaceRef   = 11
	constantNameAndType    = 12
	constantMethodHandle   = 15
	constantMethodType     = 16
	constantDynamic        = 17
	constantInvokeDynamic  = 18
	constantModule         = 19
	constantPackage        = 20
)

// ResolveJavaArchive reports Java packages and module metadata found in a JAR.
func ResolveJavaArchive(
	ctx context.Context,
	pkg provides.Package,
	reader archives.Reader,
) (provides.SurfaceResult, error) {
	if _, err := artifactIdentity(pkg, "maven"); err != nil {
		return provides.SurfaceResult{}, err
	}
	files, err := artifactFiles(ctx, reader)
	if err != nil {
		return provides.SurfaceResult{}, err
	}

	contents, err := scanJavaArchive(ctx, files)
	if err != nil {
		return provides.SurfaceResult{Surface: provides.Surface{PURL: pkg.PURL}}, err
	}

	result := provides.SurfaceResult{Surface: provides.Surface{PURL: pkg.PURL}}
	for packageName := range contents.packages {
		result.Surface.Provides = append(result.Surface.Provides, javaProvidedName(
			packageName,
			"package",
			provides.MatchPrefix,
			".",
			jarEntriesSource,
		))
	}
	moduleName, moduleSource, diagnostics := resolveJavaModule(reader, contents)
	result.Diagnostics = append(result.Diagnostics, diagnostics...)
	if moduleName != "" {
		result.Surface.Provides = append(result.Surface.Provides, javaProvidedName(
			moduleName,
			"module",
			provides.MatchExact,
			"",
			moduleSource,
		))
	}

	return provides.MergeSurfaceResults(pkg.PURL, result), nil
}

type javaArchiveContents struct {
	packages       map[string]struct{}
	moduleInfoPath string
	manifestPath   string
}

func scanJavaArchive(ctx context.Context, files []archives.FileInfo) (javaArchiveContents, error) {
	contents := javaArchiveContents{packages: make(map[string]struct{})}
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return contents, err
		}
		if file.IsDir {
			continue
		}
		if strings.EqualFold(file.Path, "META-INF/MANIFEST.MF") {
			contents.manifestPath = file.Path
		}
		classPath := javaClassPath(file.Path)
		if classPath == "module-info.class" {
			if contents.moduleInfoPath == "" || shorterArtifactPath(file.Path, contents.moduleInfoPath) {
				contents.moduleInfoPath = file.Path
			}
			continue
		}
		if strings.HasPrefix(classPath, "META-INF/") {
			continue
		}
		if !strings.HasSuffix(classPath, ".class") || !strings.Contains(classPath, "/") {
			continue
		}
		packagePath := classPath[:strings.LastIndex(classPath, "/")]
		contents.packages[strings.ReplaceAll(packagePath, "/", ".")] = struct{}{}
	}
	return contents, nil
}

func resolveJavaModule(
	reader archives.Reader,
	contents javaArchiveContents,
) (string, string, []provides.Diagnostic) {
	moduleName := ""
	moduleSource := ""
	var diagnostics []provides.Diagnostic
	if contents.moduleInfoPath != "" {
		content, readErr := readArtifactFile(reader, contents.moduleInfoPath)
		if readErr != nil {
			diagnostics = append(diagnostics, artifactDiagnostic(
				contents.moduleInfoPath,
				fmt.Sprintf("read Java module descriptor: %v", readErr),
			))
		} else if parsedName, parseErr := javaModuleName(content); parseErr != nil {
			diagnostics = append(diagnostics, artifactDiagnostic(
				contents.moduleInfoPath,
				fmt.Sprintf("parse Java module descriptor: %v", parseErr),
			))
		} else {
			moduleName = parsedName
			moduleSource = contents.moduleInfoPath
		}
	}
	if moduleName == "" && contents.manifestPath != "" {
		content, readErr := readArtifactFile(reader, contents.manifestPath)
		if readErr != nil {
			diagnostics = append(diagnostics, artifactDiagnostic(
				contents.manifestPath,
				fmt.Sprintf("read JAR manifest: %v", readErr),
			))
		} else if moduleName = javaManifestModuleName(content); moduleName != "" {
			moduleSource = contents.manifestPath
		}
	}
	return moduleName, moduleSource, diagnostics
}

func javaClassPath(filename string) string {
	const versionPrefix = "META-INF/versions/"
	if strings.HasPrefix(filename, versionPrefix) {
		remainder := strings.TrimPrefix(filename, versionPrefix)
		if slash := strings.IndexByte(remainder, '/'); slash >= 0 {
			return remainder[slash+1:]
		}
	}
	return strings.TrimPrefix(filename, "WEB-INF/classes/")
}

func javaProvidedName(
	name string,
	kind string,
	match provides.MatchMode,
	separator string,
	source string,
) provides.ProvidedName {
	return provides.ProvidedName{
		Language:  "java",
		Name:      name,
		Kind:      kind,
		Match:     match,
		Separator: separator,
		Evidence: []provides.Evidence{{
			Method: provides.EvidenceArtifact,
			Source: source,
		}},
	}
}

func javaManifestModuleName(content []byte) string {
	message, err := mail.ReadMessage(bytes.NewReader(content))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(message.Header.Get("Automatic-Module-Name"))
}

func javaModuleName(content []byte) (string, error) {
	reader := bytes.NewReader(content)
	magic, err := readUint32(reader)
	if err != nil || magic != classFileMagic {
		return "", fmt.Errorf("invalid class file")
	}
	if err := skipBytes(reader, classVersionSize); err != nil {
		return "", err
	}
	constantCount, err := readUint16(reader)
	if err != nil {
		return "", err
	}
	pool, err := readJavaConstantPool(reader, constantCount)
	if err != nil {
		return "", err
	}
	if err := skipClassMembers(reader); err != nil {
		return "", err
	}
	return readJavaModuleAttribute(reader, pool)
}

type javaConstantPool struct {
	utf8Values  map[uint16]string
	moduleNames map[uint16]uint16
}

func readJavaConstantPool(reader *bytes.Reader, constantCount uint16) (javaConstantPool, error) {
	pool := javaConstantPool{
		utf8Values:  make(map[uint16]string),
		moduleNames: make(map[uint16]uint16),
	}
	for index := uint16(1); index < constantCount; index++ {
		wide, err := readJavaConstant(reader, index, &pool)
		if err != nil {
			return javaConstantPool{}, err
		}
		if wide {
			index++
		}
	}
	return pool, nil
}

func readJavaConstant(
	reader *bytes.Reader,
	index uint16,
	pool *javaConstantPool,
) (bool, error) {
	tag, err := reader.ReadByte()
	if err != nil {
		return false, err
	}
	switch tag {
	case constantUTF8:
		length, readErr := readUint16(reader)
		if readErr != nil {
			return false, readErr
		}
		value := make([]byte, length)
		if _, readErr = io.ReadFull(reader, value); readErr != nil {
			return false, readErr
		}
		pool.utf8Values[index] = string(value)
	case constantInteger, constantFloat:
		err = skipBytes(reader, constantFourByteSize)
	case constantLong, constantDouble:
		err = skipBytes(reader, constantEightByteSize)
		return true, err
	case constantClass, constantString, constantMethodType, constantPackage:
		err = skipBytes(reader, classIndexSize)
	case constantModule:
		pool.moduleNames[index], err = readUint16(reader)
	case constantFieldRef, constantMethodRef, constantInterfaceRef,
		constantNameAndType, constantDynamic, constantInvokeDynamic:
		err = skipBytes(reader, constantFourByteSize)
	case constantMethodHandle:
		err = skipBytes(reader, constantMethodSize)
	default:
		return false, fmt.Errorf("unsupported constant-pool tag %d", tag)
	}
	return false, err
}

func readJavaModuleAttribute(reader *bytes.Reader, pool javaConstantPool) (string, error) {
	attributeCount, err := readUint16(reader)
	if err != nil {
		return "", err
	}
	for range attributeCount {
		nameIndex, readErr := readUint16(reader)
		if readErr != nil {
			return "", readErr
		}
		length, readErr := readUint32(reader)
		if readErr != nil {
			return "", readErr
		}
		if pool.utf8Values[nameIndex] != "Module" {
			if err := skipBytes(reader, int64(length)); err != nil {
				return "", err
			}
			continue
		}
		moduleIndex, readErr := readUint16(reader)
		if readErr != nil {
			return "", readErr
		}
		name := pool.utf8Values[pool.moduleNames[moduleIndex]]
		if name == "" {
			return "", fmt.Errorf("module name not found in constant pool")
		}
		return name, nil
	}
	return "", fmt.Errorf("module attribute not found")
}

func skipClassMembers(reader *bytes.Reader) error {
	if err := skipBytes(reader, classIdentitySize); err != nil {
		return err
	}
	interfaceCount, err := readUint16(reader)
	if err != nil {
		return err
	}
	if err := skipBytes(reader, int64(interfaceCount)*classIndexSize); err != nil {
		return err
	}
	for range 2 {
		memberCount, readErr := readUint16(reader)
		if readErr != nil {
			return readErr
		}
		for range memberCount {
			if err := skipBytes(reader, classMemberHeaderSize); err != nil {
				return err
			}
			attributeCount, readErr := readUint16(reader)
			if readErr != nil {
				return readErr
			}
			for range attributeCount {
				if err := skipBytes(reader, classAttributeNameSize); err != nil {
					return err
				}
				length, readErr := readUint32(reader)
				if readErr != nil {
					return readErr
				}
				if err := skipBytes(reader, int64(length)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func readUint16(reader io.Reader) (uint16, error) {
	var value uint16
	err := binary.Read(reader, binary.BigEndian, &value)
	return value, err
}

func readUint32(reader io.Reader) (uint32, error) {
	var value uint32
	err := binary.Read(reader, binary.BigEndian, &value)
	return value, err
}

func skipBytes(reader io.Reader, count int64) error {
	_, err := io.CopyN(io.Discard, reader, count)
	return err
}
