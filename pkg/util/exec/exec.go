package exec

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"k8s.io/client-go/pkg/apis/clientauthentication"
	"k8s.io/client-go/pkg/apis/clientauthentication/install"
	clientauthenticationv1 "k8s.io/client-go/pkg/apis/clientauthentication/v1"
	clientauthenticationv1beta1 "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var (
	scheme = runtime.NewScheme()

	codecs = serializer.NewCodecFactory(scheme)

	apiVersions = map[string]schema.GroupVersion{
		clientauthenticationv1beta1.SchemeGroupVersion.String(): clientauthenticationv1beta1.SchemeGroupVersion,
		clientauthenticationv1.SchemeGroupVersion.String():      clientauthenticationv1.SchemeGroupVersion,
	}
)

func init() {
	install.Install(scheme)
}

func GetToken(ec *clientcmdapi.ExecConfig, cluster *clientauthentication.Cluster) (*clientauthentication.ExecCredential, error) {
	cmd := exec.Command(ec.Command, ec.Args...)
	cmd.Env = os.Environ()

	for _, env := range ec.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", env.Name, env.Value))
	}

	var stderr, stdout bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, wrapCmdRunErrorLocked(cmd, err)
	}

	ecgv, err := schema.ParseGroupVersion(ec.APIVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to parse exec config API version: %v", err)
	}

	cred := &clientauthentication.ExecCredential{
		TypeMeta: metav1.TypeMeta{
			APIVersion: ec.APIVersion,
			Kind:       "ExecCredential",
		},
		Spec: clientauthentication.ExecCredentialSpec{
			Interactive: false,
			Cluster:     cluster,
		},
	}

	gv, ok := apiVersions[ec.APIVersion]
	if !ok {
		return nil, fmt.Errorf("exec plugin: invalid apiVersion %q", ec.APIVersion)
	}

	_, gvk, err := codecs.UniversalDecoder(gv).Decode(stdout.Bytes(), nil, cred)
	if err != nil {
		return nil, fmt.Errorf("decoding stdout: %v", err)
	}

	if gvk.Group != ecgv.Group || gvk.Version != ecgv.Version {
		return nil, fmt.Errorf("exec plugin is configured to use API version %s, plugin returned version %s", ecgv, schema.GroupVersion{Group: gvk.Group, Version: gvk.Version})
	}

	if cred.Status == nil {
		return nil, fmt.Errorf("exec plugin didn't return a status field")
	}

	if cred.Status.Token == "" && cred.Status.ClientCertificateData == "" && cred.Status.ClientKeyData == "" {
		return nil, fmt.Errorf("exec plugin didn't return a token or cert/key pair")
	}

	if (cred.Status.ClientCertificateData == "") != (cred.Status.ClientKeyData == "") {
		return nil, fmt.Errorf("exec plugin returned only certificate or key, not both")
	}

	return cred, nil
}

func wrapCmdRunErrorLocked(cmd *exec.Cmd, err error) error {
	switch err.(type) {
	case *exec.Error: // Binary does not exist (see exec.Error).
		return fmt.Errorf("exec: executable %s not found", cmd.Path)

	case *exec.ExitError: // Binary execution failed (see exec.Cmd.Run()).
		e := err.(*exec.ExitError)
		return fmt.Errorf("exec: executable %s failed with exit code %d", cmd.Path, e.ProcessState.ExitCode())

	default:
		return fmt.Errorf("exec: %v", err)
	}
}
