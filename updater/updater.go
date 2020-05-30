package updater

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var ErrUpdateHandlerNotImplemented = errors.New("update handler not implemented")

type countingWriter int64

type Updater struct {
	BaseUrl    *url.URL
	HttpClient *http.Client
}

func (cw *countingWriter) Write(p []byte) (n int, err error) {
	*cw += countingWriter(len(p))
	return len(p), nil
}

func StreamTo(updater *Updater, path string, r io.Reader) error {
	start := time.Now()
	hash := sha256.New()
	var cw countingWriter
	req, err := http.NewRequest(http.MethodPut, updater.BaseUrl.String()+path, io.TeeReader(io.TeeReader(r, hash), &cw))
	if err != nil {
		return err
	}
	resp, err := updater.HttpClient.Do(req)
	if err != nil {
		return err
	}
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("unexpected HTTP status code: got %d, want %d (body %q)", got, want, string(body))
	}
	remoteHash, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if bytes.HasPrefix(remoteHash, []byte("<!DOCTYPE html>")) {
		return ErrUpdateHandlerNotImplemented
	}
	decoded := make([]byte, hex.DecodedLen(len(remoteHash)))
	n, err := hex.Decode(decoded, remoteHash)
	if err != nil {
		return err
	}
	if got, want := decoded[:n], hash.Sum(nil); !bytes.Equal(got, want) {
		return fmt.Errorf("unexpected SHA256 hash: got %x, want %x", got, want)
	}
	duration := time.Since(start)
	// TODO: return this
	log.Printf("%d bytes in %v, i.e. %f MiB/s", int64(cw), duration, float64(cw)/duration.Seconds()/1024/1024)
	return nil
}

func UpdateRoot(updater *Updater, r io.Reader) error {
	return StreamTo(updater, "update/root", r)
}

func UpdateBoot(updater *Updater, r io.Reader) error {
	return StreamTo(updater, "update/boot", r)
}

func UpdateMBR(updater *Updater, r io.Reader) error {
	return StreamTo(updater, "update/mbr", r)
}

func Switch(updater *Updater) error {
	resp, err := updater.HttpClient.Post(updater.BaseUrl.String()+"update/switch", "", nil)
	if err != nil {
		return err
	}
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("unexpected HTTP status code: got %d, want %d (body %q)", got, want, string(body))
	}
	return nil
}

func Reboot(updater *Updater) error {
	resp, err := updater.HttpClient.Post(updater.BaseUrl.String()+"reboot", "", nil)
	if err != nil {
		return err
	}
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("unexpected HTTP status code: got %d, want %d (body %q)", got, want, string(body))
	}
	return nil
}

func TargetSupports(updater *Updater, feature string) (bool, error) {
	resp, err := updater.HttpClient.Get(updater.BaseUrl.String() + "update/features")
	if err != nil {
		return false, err
	}
	if resp.StatusCode == http.StatusNotFound {
		// Target device does not support /features handler yet, so feature
		// cannot be supported.
		return false, nil
	}
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		body, _ := ioutil.ReadAll(resp.Body)
		return false, fmt.Errorf("unexpected HTTP status code: got %d, want %d (body %q)", got, want, string(body))
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	supported := strings.Split(strings.TrimSpace(string(body)), ",")
	for _, f := range supported {
		if f == feature {
			return true, nil
		}
	}
	return false, nil
}
