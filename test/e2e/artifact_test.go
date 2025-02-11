//go:build linux || freebsd

package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/podman/v5/pkg/libartifact"
	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

const (
	//nolint:revive,stylecheck
	ARTIFACT_SINGLE = "quay.io/libpod/testartifact:20250206-single"
	//nolint:revive,stylecheck
	ARTIFACT_MULTI = "quay.io/libpod/testartifact:20250206-multi"
	//nolint:revive,stylecheck
	ARTIFACT_MULTI_NO_TITLE = "quay.io/libpod/testartifact:20250206-multi-no-title"
	//nolint:revive,stylecheck
	ARTIFACT_EVIL = "quay.io/libpod/testartifact:20250206-evil"
)

var _ = Describe("Podman artifact", func() {
	BeforeEach(func() {
		SkipIfRemote("artifacts are not supported on the remote client yet due to being in development still")
	})

	It("podman artifact ls", func() {
		artifact1File, err := createArtifactFile(4192)
		Expect(err).ToNot(HaveOccurred())
		artifact1Name := "localhost/test/artifact1"
		add1 := podmanTest.PodmanExitCleanly([]string{"artifact", "add", artifact1Name, artifact1File}...)

		artifact2File, err := createArtifactFile(10240)
		Expect(err).ToNot(HaveOccurred())
		artifact2Name := "localhost/test/artifact2"
		podmanTest.PodmanExitCleanly([]string{"artifact", "add", artifact2Name, artifact2File}...)

		// Should be three items in the list
		listSession := podmanTest.PodmanExitCleanly([]string{"artifact", "ls"}...)
		Expect(listSession.OutputToStringArray()).To(HaveLen(3))

		// --format should work
		listFormatSession := podmanTest.PodmanExitCleanly([]string{"artifact", "ls", "--format", "{{.Repository}}"}...)
		output := listFormatSession.OutputToStringArray()

		// There should be only 2 "lines" because the header should not be output
		Expect(output).To(HaveLen(2))

		// Make sure the names are what we expect
		Expect(output).To(ContainElement(artifact1Name))
		Expect(output).To(ContainElement(artifact2Name))

		// Check default digest length (should be 12)
		defaultFormatSession := podmanTest.PodmanExitCleanly([]string{"artifact", "ls", "--format", "{{.Digest}}"}...)
		defaultOutput := defaultFormatSession.OutputToStringArray()[0]
		Expect(defaultOutput).To(HaveLen(12))

		// Check with --no-trunc and verify the len of the digest is the same as the len what was returned when the artifact
		// was added
		noTruncSession := podmanTest.PodmanExitCleanly([]string{"artifact", "ls", "--no-trunc", "--format", "{{.Digest}}"}...)
		truncOutput := noTruncSession.OutputToStringArray()[0]
		Expect(truncOutput).To(HaveLen(len(add1.OutputToString())))

		// check with --noheading and verify the header is not present through a line count AND substring match
		noHeaderSession := podmanTest.PodmanExitCleanly([]string{"artifact", "ls", "--noheading"}...)
		noHeaderOutput := noHeaderSession.OutputToStringArray()
		Expect(noHeaderOutput).To(HaveLen(2))
		Expect(noHeaderOutput).ToNot(ContainElement("REPOSITORY"))

	})

	It("podman artifact simple add", func() {
		artifact1File, err := createArtifactFile(1024)
		Expect(err).ToNot(HaveOccurred())

		artifact1Name := "localhost/test/artifact1"
		podmanTest.PodmanExitCleanly([]string{"artifact", "add", artifact1Name, artifact1File}...)

		inspectSingleSession := podmanTest.PodmanExitCleanly([]string{"artifact", "inspect", artifact1Name}...)

		a := libartifact.Artifact{}
		inspectOut := inspectSingleSession.OutputToString()
		err = json.Unmarshal([]byte(inspectOut), &a)
		Expect(err).ToNot(HaveOccurred())
		Expect(a.Name).To(Equal(artifact1Name))

		// Adding an artifact with an existing name should fail
		addAgain := podmanTest.Podman([]string{"artifact", "add", artifact1Name, artifact1File})
		addAgain.WaitWithDefaultTimeout()
		Expect(addAgain).ShouldNot(ExitCleanly())
		Expect(addAgain.ErrorToString()).To(Equal(fmt.Sprintf("Error: artifact %s already exists", artifact1Name)))
	})

	It("podman artifact add with options", func() {
		artifact1Name := "localhost/test/artifact1"
		artifact1File, err := createArtifactFile(1024)
		Expect(err).ToNot(HaveOccurred())

		artifactType := "octet/foobar"
		annotation1 := "color=blue"
		annotation2 := "flavor=lemon"

		podmanTest.PodmanExitCleanly([]string{"artifact", "add", "--type", artifactType, "--annotation", annotation1, "--annotation", annotation2, artifact1Name, artifact1File}...)
		inspectSingleSession := podmanTest.PodmanExitCleanly([]string{"artifact", "inspect", artifact1Name}...)
		a := libartifact.Artifact{}
		err = json.Unmarshal([]byte(inspectSingleSession.OutputToString()), &a)
		Expect(err).ToNot(HaveOccurred())
		Expect(a.Name).To(Equal(artifact1Name))
		Expect(a.Manifest.ArtifactType).To(Equal(artifactType))
		Expect(a.Manifest.Layers[0].Annotations["color"]).To(Equal("blue"))
		Expect(a.Manifest.Layers[0].Annotations["flavor"]).To(Equal("lemon"))

		failSession := podmanTest.Podman([]string{"artifact", "add", "--annotation", "org.opencontainers.image.title=foobar", "foobar", artifact1File})
		failSession.WaitWithDefaultTimeout()
		Expect(failSession).Should(Exit(125))
		Expect(failSession.ErrorToString()).Should(Equal("Error: cannot override filename with org.opencontainers.image.title annotation"))
	})

	It("podman artifact add multiple", func() {
		artifact1File1, err := createArtifactFile(1024)
		Expect(err).ToNot(HaveOccurred())
		artifact1File2, err := createArtifactFile(8192)
		Expect(err).ToNot(HaveOccurred())

		artifact1Name := "localhost/test/artifact1"

		podmanTest.PodmanExitCleanly([]string{"artifact", "add", artifact1Name, artifact1File1, artifact1File2}...)

		inspectSingleSession := podmanTest.PodmanExitCleanly([]string{"artifact", "inspect", artifact1Name}...)

		a := libartifact.Artifact{}
		inspectOut := inspectSingleSession.OutputToString()
		err = json.Unmarshal([]byte(inspectOut), &a)
		Expect(err).ToNot(HaveOccurred())
		Expect(a.Name).To(Equal(artifact1Name))

		Expect(a.Manifest.Layers).To(HaveLen(2))
	})

	It("podman artifact push and pull", func() {
		artifact1File, err := createArtifactFile(1024)
		Expect(err).ToNot(HaveOccurred())

		lock, port, err := setupRegistry(nil)
		if err == nil {
			defer lock.Unlock()
		}
		Expect(err).ToNot(HaveOccurred())

		artifact1Name := fmt.Sprintf("localhost:%s/test/artifact1", port)
		podmanTest.PodmanExitCleanly([]string{"artifact", "add", artifact1Name, artifact1File}...)

		podmanTest.PodmanExitCleanly([]string{"artifact", "push", "-q", "--tls-verify=false", artifact1Name}...)

		podmanTest.PodmanExitCleanly([]string{"artifact", "rm", artifact1Name}...)

		podmanTest.PodmanExitCleanly([]string{"artifact", "pull", "--tls-verify=false", artifact1Name}...)

		inspectSingleSession := podmanTest.PodmanExitCleanly([]string{"artifact", "inspect", artifact1Name}...)

		a := libartifact.Artifact{}
		inspectOut := inspectSingleSession.OutputToString()
		err = json.Unmarshal([]byte(inspectOut), &a)
		Expect(err).ToNot(HaveOccurred())
		Expect(a.Name).To(Equal(artifact1Name))
	})

	It("podman artifact remove", func() {
		// Trying to remove an image that does not exist should fail
		rmFail := podmanTest.Podman([]string{"artifact", "rm", "foobar"})
		rmFail.WaitWithDefaultTimeout()
		Expect(rmFail).Should(Exit(125))
		Expect(rmFail.ErrorToString()).Should(Equal(fmt.Sprintf("Error: no artifact found with name or digest of %s", "foobar")))

		// Add an artifact to remove later
		artifact1File, err := createArtifactFile(4192)
		Expect(err).ToNot(HaveOccurred())
		artifact1Name := "localhost/test/artifact1"
		addArtifact1 := podmanTest.PodmanExitCleanly([]string{"artifact", "add", artifact1Name, artifact1File}...)

		// Removing that artifact should work
		rmWorks := podmanTest.PodmanExitCleanly([]string{"artifact", "rm", artifact1Name}...)
		// The digests printed by removal should be the same as the digest that was added
		Expect(addArtifact1.OutputToString()).To(Equal(rmWorks.OutputToString()))

		// Inspecting that the removed artifact should fail
		inspectArtifact := podmanTest.Podman([]string{"artifact", "inspect", artifact1Name})
		inspectArtifact.WaitWithDefaultTimeout()
		Expect(inspectArtifact).Should(Exit(125))
		Expect(inspectArtifact.ErrorToString()).To(Equal(fmt.Sprintf("Error: no artifact found with name or digest of %s", artifact1Name)))
	})

	It("podman artifact inspect with full or partial digest", func() {
		artifact1File, err := createArtifactFile(4192)
		Expect(err).ToNot(HaveOccurred())
		artifact1Name := "localhost/test/artifact1"
		addArtifact1 := podmanTest.PodmanExitCleanly([]string{"artifact", "add", artifact1Name, artifact1File}...)

		artifactDigest := addArtifact1.OutputToString()

		podmanTest.PodmanExitCleanly([]string{"artifact", "inspect", artifactDigest}...)
		podmanTest.PodmanExitCleanly([]string{"artifact", "inspect", artifactDigest[:12]}...)

	})

	It("podman artifact extract single", func() {
		podmanTest.PodmanExitCleanly("artifact", "pull", ARTIFACT_SINGLE)

		const (
			artifactContent = "mRuO9ykak1Q2j\n"
			artifactDigest  = "sha256:e9510923578af3632946ecf5ae479c1b5f08b47464e707b5cbab9819272a9752"
			artifactTitle   = "testfile"
		)

		path := filepath.Join(podmanTest.TempDir, "testfile")
		// Extract to non existing file
		podmanTest.PodmanExitCleanly("artifact", "extract", ARTIFACT_SINGLE, path)
		Expect(readFileToString(path)).To(Equal(artifactContent))

		// Extract to existing file will overwrite file
		path = filepath.Join(podmanTest.TempDir, "abcd")
		f, err := os.Create(path)
		Expect(err).ToNot(HaveOccurred())
		f.Close()
		podmanTest.PodmanExitCleanly("artifact", "extract", ARTIFACT_SINGLE, path)
		Expect(readFileToString(path)).To(Equal(artifactContent))

		tests := []struct {
			name      string
			filename  string
			extraArgs []string
		}{
			{
				name:     "extract to dir",
				filename: artifactTitle,
			},
			{
				name:      "extract to dir by digest",
				filename:  digestToFilename(artifactDigest),
				extraArgs: []string{"--digest", artifactDigest},
			},
			{
				name:      "extract to dir by title",
				filename:  artifactTitle,
				extraArgs: []string{"--title", artifactTitle},
			},
		}

		for _, tt := range tests {
			By(tt.name)
			dir := makeTempDirInDir(podmanTest.TempDir)
			args := append([]string{"artifact", "extract"}, tt.extraArgs...)
			args = append(args, ARTIFACT_SINGLE, dir)
			podmanTest.PodmanExitCleanly(args...)
			Expect(readFileToString(filepath.Join(dir, tt.filename))).To(Equal(artifactContent))
		}

		// invalid digest
		session := podmanTest.Podman([]string{"artifact", "extract", "--digest", "blah", ARTIFACT_SINGLE, podmanTest.TempDir})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, `no blob with the digest "blah"`))

		// invalid title
		session = podmanTest.Podman([]string{"artifact", "extract", "--title", "abcd", ARTIFACT_SINGLE, podmanTest.TempDir})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, `no blob with the title "abcd"`))
	})

	It("podman artifact extract multi", func() {
		podmanTest.PodmanExitCleanly("artifact", "pull", ARTIFACT_MULTI)
		podmanTest.PodmanExitCleanly("artifact", "pull", ARTIFACT_MULTI_NO_TITLE)

		const (
			artifactContent1 = "xuHWedtC0ADST\n"
			artifactDigest1  = "sha256:8257bba28b9d19ac353c4b713b470860278857767935ef7e139afd596cb1bb2d"
			artifactTitle1   = "test1"
			artifactContent2 = "tAyZczFlgFsi4\n"
			artifactDigest2  = "sha256:63700c54129c6daaafe3a20850079f82d6d658d69de73d6158d81f920c6fbdd7"
			artifactTitle2   = "test2"
		)

		type expect struct {
			filename string
			content  string
		}
		tests := []struct {
			name      string
			image     string
			extraArgs []string
			expect    []expect
		}{
			{
				name:  "extract multi blob to dir",
				image: ARTIFACT_MULTI,
				expect: []expect{
					{filename: artifactTitle1, content: artifactContent1},
					{filename: artifactTitle2, content: artifactContent2},
				},
			},
			{
				name:  "extract multi blob to dir without title",
				image: ARTIFACT_MULTI_NO_TITLE,
				expect: []expect{
					{filename: digestToFilename(artifactDigest1), content: artifactContent1},
					{filename: digestToFilename(artifactDigest2), content: artifactContent2},
				},
			},
			{
				name:      "extract multi blob to dir with --title",
				image:     ARTIFACT_MULTI,
				extraArgs: []string{"--title", artifactTitle1},
				expect: []expect{
					{filename: artifactTitle1, content: artifactContent1},
				},
			},
			{
				name:      "extract multi blob to dir with --digest",
				image:     ARTIFACT_MULTI,
				extraArgs: []string{"--digest", artifactDigest2},
				expect: []expect{
					{filename: digestToFilename(artifactDigest2), content: artifactContent2},
				},
			},
		}

		for _, tt := range tests {
			By(tt.name)
			dir := makeTempDirInDir(podmanTest.TempDir)
			args := append([]string{"artifact", "extract"}, tt.extraArgs...)
			args = append(args, tt.image, dir)
			podmanTest.PodmanExitCleanly(args...)
			files, err := os.ReadDir(dir)
			Expect(err).ToNot(HaveOccurred())
			Expect(files).To(HaveLen(len(tt.expect)))
			for _, expect := range tt.expect {
				Expect(readFileToString(filepath.Join(dir, expect.filename))).To(Equal(expect.content))
			}
		}
	})

	It("podman artifact extract evil", func() {
		path := filepath.Join(podmanTest.TempDir, "testfile")
		podmanTest.PodmanExitCleanly("artifact", "pull", ARTIFACT_EVIL)

		const (
			artifactContent = "RM5eA27F9psa2\n"
			artifactDigest  = "sha256:4c29da41ff27fcbf273653bcfba58ed69efa4aefec7b6c486262711cb1dfd050"
		)

		// Extract to file is fine as we are not using the malicious title
		podmanTest.PodmanExitCleanly("artifact", "extract", ARTIFACT_EVIL, path)
		Expect(readFileToString(path)).To(Equal(artifactContent))

		// This must fail for security reasons we do not allow a title with /
		session := podmanTest.Podman([]string{"artifact", "extract", ARTIFACT_EVIL, podmanTest.TempDir})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, `invalid name: "../../../../tmp/evil" cannot contain /`))

		// Extracting by digest should be fine too
		podmanTest.PodmanExitCleanly("artifact", "extract", "--digest", artifactDigest, ARTIFACT_EVIL, podmanTest.TempDir)
		Expect(readFileToString(filepath.Join(podmanTest.TempDir, digestToFilename(artifactDigest)))).To(Equal(artifactContent))
	})
})

func digestToFilename(digest string) string {
	return strings.ReplaceAll(digest, ":", "-")
}

func readFileToString(path string) string {
	GinkgoHelper()
	b, err := os.ReadFile(path)
	Expect(err).ToNot(HaveOccurred())
	return string(b)
}
