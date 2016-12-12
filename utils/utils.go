package utils

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/docker/distribution/registry/api/errcode"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/stringid"
)

// SelfPath figures out the absolute path of our own binary (if it's still around).
func SelfPath() string {
	path, err := exec.LookPath(os.Args[0])
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		if execErr, ok := err.(*exec.Error); ok && os.IsNotExist(execErr.Err) {
			return ""
		}
		panic(err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		panic(err)
	}
	return path
}

func dockerInitSha1(target string) string {
	f, err := os.Open(target)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha1.New()
	_, err = io.Copy(h, f)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

func isValidDockerInitPath(target string, selfPath string) bool { // target and selfPath should be absolute (InitPath and SelfPath already do this)
	if target == "" {
		return false
	}
	if dockerversion.IAmStatic == "true" {
		if selfPath == "" {
			return false
		}
		if target == selfPath {
			return true
		}
		targetFileInfo, err := os.Lstat(target)
		if err != nil {
			return false
		}
		selfPathFileInfo, err := os.Lstat(selfPath)
		if err != nil {
			return false
		}
		return os.SameFile(targetFileInfo, selfPathFileInfo)
	}
	return dockerversion.InitSHA1 != "" && dockerInitSha1(target) == dockerversion.InitSHA1
}

// DockerInitPath figures out the path of our dockerinit (which may be SelfPath())
func DockerInitPath(localCopy string) string {
	selfPath := SelfPath()
	if isValidDockerInitPath(selfPath, selfPath) {
		// if we're valid, don't bother checking anything else
		return selfPath
	}
	var possibleInits = []string{
		localCopy,
		dockerversion.InitPath,
		filepath.Join(filepath.Dir(selfPath), "dockerinit"),

		// FHS 3.0 Draft: "/usr/libexec includes internal binaries that are not intended to be executed directly by users or shell scripts. Applications may use a single subdirectory under /usr/libexec."
		// https://www.linuxbase.org/betaspecs/fhs/fhs.html#usrlibexec
		"/usr/libexec/docker/dockerinit",
		"/usr/local/libexec/docker/dockerinit",

		// FHS 2.3: "/usr/lib includes object files, libraries, and internal binaries that are not intended to be executed directly by users or shell scripts."
		// https://refspecs.linuxfoundation.org/FHS_2.3/fhs-2.3.html#USRLIBLIBRARIESFORPROGRAMMINGANDPA
		"/usr/lib/docker/dockerinit",
		"/usr/local/lib/docker/dockerinit",
	}
	for _, dockerInit := range possibleInits {
		if dockerInit == "" {
			continue
		}
		path, err := exec.LookPath(dockerInit)
		if err == nil {
			path, err = filepath.Abs(path)
			if err != nil {
				// LookPath already validated that this file exists and is executable (following symlinks), so how could Abs fail?
				panic(err)
			}
			if isValidDockerInitPath(path, selfPath) {
				return path
			}
		}
	}
	return ""
}

var globalTestID string

// TestDirectory creates a new temporary directory and returns its path.
// The contents of directory at path `templateDir` is copied into the
// new directory.
func TestDirectory(templateDir string) (dir string, err error) {
	if globalTestID == "" {
		globalTestID = stringid.GenerateNonCryptoID()[:4]
	}
	prefix := fmt.Sprintf("docker-test%s-%s-", globalTestID, GetCallerName(2))
	if prefix == "" {
		prefix = "docker-test-"
	}
	dir, err = ioutil.TempDir("", prefix)
	if err = os.Remove(dir); err != nil {
		return
	}
	if templateDir != "" {
		if err = archive.CopyWithTar(templateDir, dir); err != nil {
			return
		}
	}
	return
}

// GetCallerName introspects the call stack and returns the name of the
// function `depth` levels down in the stack.
func GetCallerName(depth int) string {
	// Use the caller function name as a prefix.
	// This helps trace temp directories back to their test.
	pc, _, _, _ := runtime.Caller(depth + 1)
	callerLongName := runtime.FuncForPC(pc).Name()
	parts := strings.Split(callerLongName, ".")
	callerShortName := parts[len(parts)-1]
	return callerShortName
}

// ReplaceOrAppendEnvValues returns the defaults with the overrides either
// replaced by env key or appended to the list
func ReplaceOrAppendEnvValues(defaults, overrides []string) []string {
	cache := make(map[string]int, len(defaults))
	for i, e := range defaults {
		parts := strings.SplitN(e, "=", 2)
		cache[parts[0]] = i
	}

	for _, value := range overrides {
		// Values w/o = means they want this env to be removed/unset.
		if !strings.Contains(value, "=") {
			if i, exists := cache[value]; exists {
				defaults[i] = "" // Used to indicate it should be removed
			}
			continue
		}

		// Just do a normal set/update
		parts := strings.SplitN(value, "=", 2)
		if i, exists := cache[parts[0]]; exists {
			defaults[i] = value
		} else {
			defaults = append(defaults, value)
		}
	}

	// Now remove all entries that we want to "unset"
	for i := 0; i < len(defaults); i++ {
		if defaults[i] == "" {
			defaults = append(defaults[:i], defaults[i+1:]...)
			i--
		}
	}

	return defaults
}

// GetErrorMessage returns the human readable message associated with
// the passed-in error. In some cases the default Error() func returns
// something that is less than useful so based on its types this func
// will go and get a better piece of text.
func GetErrorMessage(err error) string {
	switch err.(type) {
	case errcode.Error:
		e, _ := err.(errcode.Error)
		return e.Message

	case errcode.ErrorCode:
		ec, _ := err.(errcode.ErrorCode)
		return ec.Message()

	default:
		return err.Error()
	}
}
