package httpclient

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/gokrazy/internal/config"
	"github.com/gokrazy/internal/updater"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func GetUpdaterByTLSFlag(tlsFlag *string, baseUrl *url.URL) (*updater.Updater, bool, error) {
	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		log.Printf("initializing x509 system cert pool failed (%v), falling back to empty cert pool", err)
	}
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}

	foundMatchingCertificate := false
	// Append user specified certificate(s)
	if *tlsFlag != "self-signed" {
		usrCert := strings.Split(*tlsFlag, ",")[0]
		certBytes, err := ioutil.ReadFile(usrCert)
		if err != nil {
			return nil, false, fmt.Errorf("reading user specified certificate %s: %v", usrCert, err)
		}
		rootCAs.AppendCertsFromPEM(certBytes)
	} else {
		// Try to find a certificate in the local host config
		hostConfig := config.HostnameSpecific(baseUrl.Host)
		certPath := filepath.Join(string(hostConfig), "cert.pem")
		if _, err := os.Stat(certPath); !os.IsNotExist(err) {
			foundMatchingCertificate = true
			log.Printf("Using certificate %s", certPath)
			certBytes, err := ioutil.ReadFile(certPath)
			if err != nil {
				return nil, false, fmt.Errorf("reading certificate %s: %v", certPath, err)
			}
			rootCAs.AppendCertsFromPEM(certBytes)
		}
	}
	client, err := GetTLSHttpClient(rootCAs)
	if err != nil {
		return nil, false, err
	}
	return &updater.Updater{BaseUrl: baseUrl, HttpClient: client}, foundMatchingCertificate, nil
}

func GetTLSHttpClient(trustStore *x509.CertPool) (*http.Client, error) {
	tlsClientConfig := &tls.Config{
		RootCAs: trustStore,
	}
	httpTransport := http.DefaultTransport.(*http.Transport).Clone()
	httpTransport.TLSClientConfig = tlsClientConfig
	httpClient := &http.Client{
		Transport: httpTransport,
	}
	return httpClient, nil
}

func GetRemoteScheme(baseUrl *url.URL) (string, error) {
	// probe for https redirect, before sending credentials via http
	probeClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	probeResp, err := probeClient.Get("http://" + baseUrl.Host)
	if err != nil {
		return "", fmt.Errorf("probing url for https: %v", err)
	}
	probeLocation, err := probeResp.Location()
	if err != nil {
		return "", fmt.Errorf("getting probe url for https: %v", err)
	}
	return probeLocation.Scheme, nil
}
