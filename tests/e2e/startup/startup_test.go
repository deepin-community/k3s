package startup

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/tests/e2e"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Valid nodeOS: generic/ubuntu2004, opensuse/Leap-15.3.x86_64
var nodeOS = flag.String("nodeOS", "generic/ubuntu2004", "VM operating system")
var ci = flag.Bool("ci", false, "running on CI")

// Environment Variables Info:
// E2E_RELEASE_VERSION=v1.23.1+k3s2 or nil for latest commit from master

// This test suite is used to verify that K3s can start up with dynamic configurations that require
// both server and agent nodes. It is unique in passing dynamic arguments to vagrant, unlike the
// rest of the E2E tests, which use static Vagrantfiles and cluster configurations.
// If you have a server only flag, the startup integration test is a better place to test it.

func Test_E2EStartupValidation(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Startup Test Suite", suiteConfig, reporterConfig)
}

var (
	kubeConfigFile  string
	serverNodeNames []string
	agentNodeNames  []string
)

func StartK3sCluster(nodes []string, serverYAML string, agentYAML string) error {

	for _, node := range nodes {
		var yamlCmd string
		var resetCmd string
		var startCmd string
		if strings.Contains(node, "server") {
			resetCmd = "sudo head -n 3 /etc/rancher/k3s/config.yaml > /tmp/config.yaml && sudo mv /tmp/config.yaml /etc/rancher/k3s/config.yaml"
			yamlCmd = fmt.Sprintf("sudo echo '%s' >> /etc/rancher/k3s/config.yaml", serverYAML)
			startCmd = "sudo systemctl start k3s"
		} else {
			resetCmd = "sudo head -n 4 /etc/rancher/k3s/config.yaml > /tmp/config.yaml && sudo mv /tmp/config.yaml /etc/rancher/k3s/config.yaml"
			yamlCmd = fmt.Sprintf("sudo echo '%s' >> /etc/rancher/k3s/config.yaml", agentYAML)
			startCmd = "sudo systemctl start k3s-agent"
		}
		if _, err := e2e.RunCmdOnNode(resetCmd, node); err != nil {
			return err
		}
		if _, err := e2e.RunCmdOnNode(yamlCmd, node); err != nil {
			return err
		}
		if _, err := e2e.RunCmdOnNode(startCmd, node); err != nil {
			return &e2e.NodeError{Node: node, Cmd: startCmd, Err: err}
		}
	}
	return nil
}

func KillK3sCluster(nodes []string) error {
	for _, node := range nodes {
		if _, err := e2e.RunCmdOnNode("sudo k3s-killall.sh", node); err != nil {
			return err
		}
	}
	return nil
}

var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Various Startup Configurations", Ordered, func() {
	Context("Verify CRI-Dockerd :", func() {
		It("Stands up the nodes", func() {
			var err error
			serverNodeNames, agentNodeNames, err = e2e.CreateLocalCluster(*nodeOS, 1, 1)
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
		})
		It("Starts K3s with no issues", func() {
			dockerYAML := "docker: true"
			err := StartK3sCluster(append(serverNodeNames, agentNodeNames...), dockerYAML, dockerYAML)
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))

			fmt.Println("CLUSTER CONFIG")
			fmt.Println("OS:", *nodeOS)
			fmt.Println("Server Nodes:", serverNodeNames)
			fmt.Println("Agent Nodes:", agentNodeNames)
			kubeConfigFile, err = e2e.GenKubeConfigFile(serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
		})

		It("Checks node and pod status", func() {
			fmt.Printf("\nFetching node status\n")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "620s", "5s").Should(Succeed())
			_, _ = e2e.ParseNodes(kubeConfigFile, true)

			fmt.Printf("\nFetching pods status\n")
			Eventually(func(g Gomega) {
				pods, err := e2e.ParsePods(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, pod := range pods {
					if strings.Contains(pod.Name, "helm-install") {
						g.Expect(pod.Status).Should(Equal("Completed"), pod.Name)
					} else {
						g.Expect(pod.Status).Should(Equal("Running"), pod.Name)
					}
				}
			}, "620s", "5s").Should(Succeed())
			_, _ = e2e.ParsePods(kubeConfigFile, true)
		})
		It("Kills the cluster", func() {
			err := KillK3sCluster(append(serverNodeNames, agentNodeNames...))
			Expect(err).NotTo(HaveOccurred())
		})
	})
	Context("Verify prefer-bundled-bin flag", func() {
		It("Starts K3s with no issues", func() {
			preferBundledYAML := "prefer-bundled-bin: true"
			err := StartK3sCluster(append(serverNodeNames, agentNodeNames...), preferBundledYAML, preferBundledYAML)
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))

			fmt.Println("CLUSTER CONFIG")
			fmt.Println("OS:", *nodeOS)
			fmt.Println("Server Nodes:", serverNodeNames)
			fmt.Println("Agent Nodes:", agentNodeNames)
			kubeConfigFile, err = e2e.GenKubeConfigFile(serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
		})

		It("Checks node and pod status", func() {
			fmt.Printf("\nFetching node status\n")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "620s", "5s").Should(Succeed())
			_, _ = e2e.ParseNodes(kubeConfigFile, true)

			fmt.Printf("\nFetching pods status\n")
			Eventually(func(g Gomega) {
				pods, err := e2e.ParsePods(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, pod := range pods {
					if strings.Contains(pod.Name, "helm-install") {
						g.Expect(pod.Status).Should(Equal("Completed"), pod.Name)
					} else {
						g.Expect(pod.Status).Should(Equal("Running"), pod.Name)
					}
				}
			}, "620s", "5s").Should(Succeed())
			_, _ = e2e.ParsePods(kubeConfigFile, true)
		})
		It("Kills the cluster", func() {
			err := KillK3sCluster(append(serverNodeNames, agentNodeNames...))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if failed && !*ci {
		fmt.Println("FAILED!")
	} else {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(kubeConfigFile)).To(Succeed())
	}
})
