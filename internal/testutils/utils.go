// Copyright 2020 The Operator-SDK Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testutils

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	kbtestutils "sigs.k8s.io/kubebuilder/v3/test/e2e/utils"
)

const BinaryName = "operator-sdk"

// TestContext wraps kubebuilder's e2e TestContext.
type TestContext struct {
	*kbtestutils.TestContext
	// BundleImageName store the image to use to build the bundle
	BundleImageName string
	// ProjectName store the project name
	ProjectName string
	// isPrometheusManagedBySuite is true when the suite tests is installing/uninstalling the Prometheus
	isPrometheusManagedBySuite bool
	// isOLMManagedBySuite is true when the suite tests is installing/uninstalling the OLM
	isOLMManagedBySuite bool
}

// NewTestContext returns a TestContext containing a new kubebuilder TestContext.
// Construct if your environment is connected to a live cluster, ex. for e2e tests.
func NewTestContext(binaryName string, env ...string) (tc TestContext, err error) {
	if tc.TestContext, err = kbtestutils.NewTestContext(binaryName, env...); err != nil {
		return tc, err
	}
	tc.ProjectName = strings.ToLower(filepath.Base(tc.Dir))
	tc.ImageName = makeImageName(tc.ProjectName)
	tc.BundleImageName = makeBundleImageName(tc.ProjectName)
	tc.isOLMManagedBySuite = true
	tc.isPrometheusManagedBySuite = true
	return tc, nil
}

// NewPartialTestContext returns a TestContext containing a partial kubebuilder TestContext.
// This object needs to be populated with GVK information. The underlying TestContext is
// created directly rather than through a constructor so cluster-based setup is skipped.
func NewPartialTestContext(binaryName, dir string, env ...string) (tc TestContext, err error) {
	cc := &kbtestutils.CmdContext{
		Env: env,
	}
	if cc.Dir, err = filepath.Abs(dir); err != nil {
		return tc, err
	}
	projectName := strings.ToLower(filepath.Base(dir))

	return TestContext{
		TestContext: &kbtestutils.TestContext{
			CmdContext: cc,
			BinaryName: binaryName,
			ImageName:  makeImageName(projectName),
		},
		ProjectName:     projectName,
		BundleImageName: makeBundleImageName(projectName),
	}, nil
}

func makeImageName(projectName string) string {
	return fmt.Sprintf("quay.io/example/%s:v0.0.1", projectName)
}

func makeBundleImageName(projectName string) string {
	return fmt.Sprintf("quay.io/example/%s-bundle:v0.0.1", projectName)
}

// InstallOLM runs 'operator-sdk olm install' for specific version
// and returns any errors emitted by that command.
func (tc TestContext) InstallOLMVersion(version string) error {
	cmd := exec.Command(tc.BinaryName, "olm", "install", "--version", version, "--timeout", "4m")
	_, err := tc.Run(cmd)
	return err
}

// InstallOLM runs 'operator-sdk olm uninstall' and logs any errors emitted by that command.
func (tc TestContext) UninstallOLM() {
	cmd := exec.Command(tc.BinaryName, "olm", "uninstall")
	if _, err := tc.Run(cmd); err != nil {
		fmt.Fprintln(GinkgoWriter, "warning: error when uninstalling OLM:", err)
	}
}

// ReplaceInFile replaces all instances of old with new in the file at path.
// todo(camilamacedo86): this func can be pushed to upstream/kb
func ReplaceInFile(path, old, new string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	if !strings.Contains(string(b), old) {
		return errors.New("unable to find the content to be replaced")
	}
	s := strings.Replace(string(b), old, new, -1)
	err = ioutil.WriteFile(path, []byte(s), info.Mode())
	if err != nil {
		return err
	}
	return nil
}

// ReplaceRegexInFile finds all strings that match `match` and replaces them
// with `replace` in the file at path.
// todo(camilamacedo86): this func can be pushed to upstream/kb
func ReplaceRegexInFile(path, match, replace string) error {
	matcher, err := regexp.Compile(match)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	s := matcher.ReplaceAllString(string(b), replace)
	if s == string(b) {
		return errors.New("unable to find the content to be replaced")
	}
	err = ioutil.WriteFile(path, []byte(s), info.Mode())
	if err != nil {
		return err
	}
	return nil
}

// LoadImageToKindClusterWithName loads a local docker image with the name informed to the kind cluster
func (tc TestContext) LoadImageToKindClusterWithName(image string) error {
	cluster := "kind"
	if v, ok := os.LookupEnv("KIND_CLUSTER"); ok {
		cluster = v
	}
	kindOptions := []string{"load", "docker-image", "--name", cluster, image}
	cmd := exec.Command("kind", kindOptions...)
	_, err := tc.Run(cmd)
	return err
}

// UncommentCode searches for target in the file and remove the comment prefix
// of the target content. The target content may span multiple lines.
// todo(camilamacedo86): this func exists in upstream/kb but there the error is not thrown. We need to
// push this change. See: https://github.com/kubernetes-sigs/kubebuilder/blob/master/test/e2e/utils/util.go
func UncommentCode(filename, target, prefix string) error {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	strContent := string(content)

	idx := strings.Index(strContent, target)
	if idx < 0 {
		// todo: push this check to upstream for we do not need have this func here
		return fmt.Errorf("unable to find the code %s to be uncomment", target)
	}

	out := new(bytes.Buffer)
	_, err = out.Write(content[:idx])
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(bytes.NewBufferString(target))
	if !scanner.Scan() {
		return nil
	}
	for {
		_, err := out.WriteString(strings.TrimPrefix(scanner.Text(), prefix))
		if err != nil {
			return err
		}
		// Avoid writing a newline in case the previous line was the last in target.
		if !scanner.Scan() {
			break
		}
		if _, err := out.WriteString("\n"); err != nil {
			return err
		}
	}

	_, err = out.Write(content[idx+len(target):])
	if err != nil {
		return err
	}
	// false positive
	// nolint:gosec
	return ioutil.WriteFile(filename, out.Bytes(), 0644)
}

// InstallPrerequisites will install OLM and Prometheus
// when the cluster kind is Kind and when they are not present on the Cluster
func (tc TestContext) InstallPrerequisites() {
	By("checking API resources applied on Cluster")
	output, err := tc.Kubectl.Command("api-resources")
	Expect(err).NotTo(HaveOccurred())
	if strings.Contains(output, "servicemonitors") {
		tc.isPrometheusManagedBySuite = false
	}
	if strings.Contains(output, "clusterserviceversions") {
		tc.isOLMManagedBySuite = false
	}

	if tc.isPrometheusManagedBySuite {
		By("installing Prometheus")
		Expect(tc.InstallPrometheusOperManager()).To(Succeed())

		By("ensuring provisioned Prometheus Manager Service")
		Eventually(func() error {
			_, err := tc.Kubectl.Get(
				false,
				"Service", "prometheus-operator")
			return err
		}, 3*time.Minute, time.Second).Should(Succeed())
	}

	if tc.isOLMManagedBySuite {
		By("installing OLM")
		Expect(tc.InstallOLMVersion(OlmVersionForTestSuite)).To(Succeed())
	}
}

// IsRunningOnKind returns true when the tests are executed in a Kind Cluster
func (tc TestContext) IsRunningOnKind() (bool, error) {
	kubectx, err := tc.Kubectl.Command("config", "current-context")
	if err != nil {
		return false, err
	}
	return strings.Contains(kubectx, "kind"), nil
}

// UninstallPrerequisites will uninstall all prerequisites installed via InstallPrerequisites()
func (tc TestContext) UninstallPrerequisites() {
	if tc.isPrometheusManagedBySuite {
		By("uninstalling Prometheus")
		tc.UninstallPrometheusOperManager()
	}
	if tc.isOLMManagedBySuite {
		By("uninstalling OLM")
		tc.UninstallOLM()
	}
}

// AllowProjectBeMultiGroup will update the PROJECT file with the information to allow we scaffold
// apis with different groups.
// todo(camilamacedo86) : remove this helper when the edit plugin via the bin
// be available. See the Pr: https://github.com/kubernetes-sigs/kubebuilder/pull/1691
func (tc TestContext) AllowProjectBeMultiGroup() error {
	const multiGroup = `multigroup: true
`
	projectBytes, err := ioutil.ReadFile(filepath.Join(tc.Dir, "PROJECT"))
	if err != nil {
		return err
	}

	projectBytes = append([]byte(multiGroup), projectBytes...)
	err = ioutil.WriteFile(filepath.Join(tc.Dir, "PROJECT"), projectBytes, 0644)
	if err != nil {
		return err
	}
	return nil
}

// WrapWarnOutput is a one-liner to wrap an error from a command that returns (string, error) in a warning.
func WrapWarnOutput(_ string, err error) {
	if err != nil {
		fmt.Fprintf(GinkgoWriter, "warning: %s", err)
	}
}

// WrapWarn is a one-liner to wrap an error from a command that returns (error) in a warning.
func WrapWarn(err error) {
	WrapWarnOutput("", err)
}
