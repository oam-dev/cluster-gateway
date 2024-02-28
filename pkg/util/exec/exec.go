package exec

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

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

	credentials sync.Map
)

func init() {
	install.Install(scheme)
}

func IssueClusterCredential(name string, ec *clientcmdapi.ExecConfig) (*clientauthentication.ExecCredential, error) {
	if name == "" {
		return nil, errors.New("cluster name not provided")
	}

	value, found := credentials.Load(name)
	if found {
		cred, ok := value.(*clientauthentication.ExecCredential)
		if !ok {
			return nil, errors.New("failed to convert item in cache to ExecCredential")
		}

		now := &metav1.Time{Time: time.Now().Add(time.Minute)} // expires a minute early

		if cred.Status != nil && cred.Status.ExpirationTimestamp.Before(now) {
			credentials.Delete(name)
			return IssueClusterCredential(name, ec) // credential expired, calling function again
		}

		return cred, nil
	}

	cred, err := issueClusterCredential(ec)
	if err != nil {
		return nil, err
	}

	if cred.Status != nil && !cred.Status.ExpirationTimestamp.IsZero() {
		credentials.Store(name, cred) // storing credential in cache
	}

	return cred, nil
}

func issueClusterCredential(ec *clientcmdapi.ExecConfig) (*clientauthentication.ExecCredential, error) {
	if ec == nil {
		return nil, errors.New("exec config not provided")
	}

	if ec.Command == "" {
		return nil, errors.New("missing \"command\" property on exec config object")
	}

	command, err := exec.LookPath(ec.Command)
	if err != nil {
		return nil, unwrapExecCommandError(ec.Command, err)
	}

	cmd := exec.Command(command, ec.Args...)
	cmd.Env = os.Environ()

	for _, env := range ec.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", env.Name, env.Value))
	}

	var stderr, stdout bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, unwrapExecCommandError(command, err)
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
		Spec: clientauthentication.ExecCredentialSpec{},
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

func unwrapExecCommandError(path string, err error) error {
	switch err.(type) {
	case *exec.Error: // Binary does not exist (see exec.Error).
		return fmt.Errorf("exec: executable %s not found", path)

	case *exec.ExitError: // Binary execution failed (see exec.Cmd.Run()).
		e := err.(*exec.ExitError)
		return fmt.Errorf("exec: executable %s failed with exit code %d", path, e.ProcessState.ExitCode())

	default:
		return fmt.Errorf("exec: %v", err)
	}
}
