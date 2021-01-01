package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

func main() {
	// Update and collect all current tenant credentials
	tcreds, err := AllTenantCredentials()
	if err != nil {
		log.Printf("\n%v\n")
	}
	// Update and collect all current proxy credentials
	pcreds, err := AllProxyCredentials()
	if err != nil {
		log.Printf("\n%v\n")
	}

	// create a slice with all the tenant usernames
	// slice is used to compare to the current proxy users
	var usernames []string
	for _, pc := range pcreds.Users {
		usernames = append(usernames, pc.Username)
	}

	// compare tenant credentials with proxy credentials
	// apply new credentials to the proxy credentials
	var newcreds Users
	for _, tc := range tcreds {
		b := Contains(usernames, tc.Client.BasicAuth.Username)
		if b != true {
			newcreds.Username = tc.Client.BasicAuth.Username
			newcreds.Password = tc.Client.BasicAuth.Password
			newcreds.Orgid = tc.Client.BasicAuth.Username
			pcreds.Users = append(pcreds.Users, newcreds)
		}
	}

	// update proxy kubernetes secret
	ReplaceProxySecret(os.Getenv("K8S_PROXY_SECRET_NAMESPACE"), "authn.yaml", pcreds)
	// restart proxy
	RestartPod(os.Getenv("K8S_PROXY_SECRET_NAMESPACE"), os.Getenv("K8S_PROXY_POD_NAME"))
	fmt.Printf("\nproxy has been restarted\n")

	// test by getting the credentials from the current proxy and tenant secrets
	TestGetProxyCredentials()
}

func Contains(source []string, value string) bool {
	for _, item := range source {
		if item == value {
			return true
		}
	}
	return false
}

func RestartPod(namespace, podname string) {
	// initiate kube client
	var kube KubeCLient
	// restart proxy pod by deleting pod
	// the replicaset will create a new pod with updated config
	pods := kube.GetAllPodNames(kube.CreateClientSet(), namespace)
	for _, p := range pods {
		if strings.Contains(p, podname) {
			kube.DeletePod(kube.CreateClientSet(), namespace, p)
		}
	}
}

// collects all proxy credentials
func AllProxyCredentials() (ProxyCredentials, error) {
	var err error
	// import environment variables
	proxysec := os.Getenv("K8S_PROXY_SECRET_NAME")
	proxyns := os.Getenv("K8S_PROXY_SECRET_NAMESPACE")
	// initiate kube client
	var kube KubeCLient

	// get the proxy credentials
	proxycred, err := GetProxyCredentials(string(
		kube.GetSecretData(kube.CreateClientSet(),
			proxyns, proxysec, "authn.yaml")))
	if err != nil {
		return proxycred, err
	}
	return proxycred, err
}

// collects all tenant credentials
// updates credentials when password is an empty string
func AllTenantCredentials() ([]TenantCredential, error) {
	var err error
	// import environment variable
	tenantsec := os.Getenv("K8S_TENANT_SECRET_NAME")
	// initiate kube client
	var kube KubeCLient
	//set slice of tenant credential
	var tcreds []TenantCredential

	namespaces := kube.GetAllNamespaceNames(kube.CreateClientSet())
	for _, ns := range namespaces {
		var c TenantCredential
		s := kube.GetSecretData(kube.CreateClientSet(),
			ns, tenantsec, "promtail.yaml")
		if len(s) != 0 {
			err = yaml.Unmarshal(s, &c)
			if err != nil {
				return nil, err
			}
			if len(c.Client.BasicAuth.Password) == 0 {
				ReplaceTenantSecret(ns, "promtail.yaml")
				// get updated tenant credential
				// append updated credentials to slice of tenant credential
				upd, err := GetTenantCredential(string(kube.GetSecretData(
					kube.CreateClientSet(), ns, tenantsec, "promtail.yaml")))
				if err != nil {
					return nil, err
				}
				tcreds = append(tcreds, upd)
			}
		}
	}
	return tcreds, err
}
