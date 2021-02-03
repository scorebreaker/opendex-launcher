package core

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
)

var (
	ErrNotFound = errors.New("not found")

	ReleaseRef = regexp.MustCompile(`^\d{2}\.\d{2}\.\d{2}.*$`)
)

type GitHub struct {
	Client      *http.Client
	Logger      *logrus.Entry
	AccessToken string
}

func NewGitHub(accessToken string) *GitHub {
	return &GitHub{
		Client:      http.DefaultClient,
		Logger:      logrus.NewEntry(logrus.StandardLogger()).WithField("name", "github"),
		AccessToken: accessToken,
	}
}

func (t *GitHub) getResponseError(resp *http.Response) error {
	var err error
	if resp.StatusCode != http.StatusOK {
		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		if err != nil {
			return fmt.Errorf("decode: %w", err)
		}
		return errors.New(result["message"].(string))
	}
	return nil
}

func (t *GitHub) doGet(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", "application/vnd.github.v3+json")
	resp, err := t.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := t.getResponseError(resp); err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (t *GitHub) GetHeadCommit(branch string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/opendexnetwork/opendex-docker/commits/%s", branch)
	body, err := t.doGet(url)
	if err != nil {
		return "", err
	}
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", err
	}
	return result["sha"].(string), nil
}

type Artifact struct {
	Name               string `json:"name"`
	SizeInBytes        uint   `json:"size_in_bytes"`
	ArchiveDownloadUrl string `json:"archive_download_url"`
}

type ArtifactList struct {
	TotalCount uint       `json:"total_count"`
	Artifacts  []Artifact `json:"artifacts"`
}

type WorkflowRun struct {
	Id         uint   `json:"id"`
	CreatedAt  string `json:"created_at"`
	HeadBranch string `json:"head_branch"`
	HeadSha    string `json:"head_sha"`
}

type WorkflowRunList struct {
	TotalCount   uint          `json:"total_count"`
	WorkflowRuns []WorkflowRun `json:"workflow_runs"`
}

func (t *GitHub) getDownloadUrl(runId uint) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/opendexnetwork/opendex-docker/actions/runs/%d/artifacts", runId)
	body, err := t.doGet(url)
	if err != nil {
		return "", err
	}
	var result ArtifactList
	err = json.Unmarshal(body, &result)
	for _, artifact := range result.Artifacts {
		name := fmt.Sprintf("%s-amd64", runtime.GOOS)
		if name == artifact.Name {
			return artifact.ArchiveDownloadUrl, nil
		}
	}
	return "", ErrNotFound
}

func (t *GitHub) getLastRunOfBranch(branch string, commit string) (*WorkflowRun, error) {
	url := fmt.Sprintf("https://api.github.com/repos/opendexnetwork/opendex-docker/actions/workflows/build.yml/runs?branch=%s", branch)
	body, err := t.doGet(url)
	if err != nil {
		return nil, err
	}
	var result WorkflowRunList
	err = json.Unmarshal(body, &result)
	if len(result.WorkflowRuns) == 0 {
		return nil, ErrNotFound
	}
	run := &result.WorkflowRuns[0]
	if run.HeadSha != commit {
		return nil, ErrNotFound
	}
	return run, nil
}

func (t *GitHub) DownloadLatestBinary(branch string, commit string) error {
	var err error
	var url string

	if ReleaseRef.Match([]byte(branch)) {
		url = fmt.Sprintf("https://github.com/opendexnetwork/opendex-docker/releases/download/%s/launcher-%s-%s.zip", branch, runtime.GOOS, runtime.GOARCH)
	} else {
		run, err := t.getLastRunOfBranch(branch, commit)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return fmt.Errorf("no launcher build for commit %s (The branch \"%s\" does not have a binary launcher)", commit, branch)
			}
			return fmt.Errorf("get last run of branch: %w", err)
		}

		url, err = t.getDownloadUrl(run.Id)
		if err != nil {
			return fmt.Errorf("get download url: %w", err)
		}
		t.Logger.Debugf("Download launcher.zip from %s", url)
	}

	if _, err := os.Stat(commit); os.IsNotExist(err) {
		err = os.Mkdir(commit, 0755)
		if err != nil {
			return fmt.Errorf("create commit (%s) folder: %w", commit, err)
		}
	}

	err = os.Chdir(commit)
	if err != nil {
		return fmt.Errorf("change directory: %w", err)
	}

	err = t.downloadFile(url, "launcher.zip")
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}

	err = t.unzip("launcher.zip")
	if err != nil {
		return fmt.Errorf("unzip: %w", err)
	}

	return nil
}

func (t *GitHub) unzip(file string) error {
	var filenames []string

	r, err := zip.OpenReader(file)
	if err != nil {
		return fmt.Errorf("open reader: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		t.Logger.Debugf("Extracting %s", f.Name)

		fpath := f.Name

		filenames = append(filenames, fpath)

		if f.FileInfo().IsDir() {
			// Make Folder
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		// Make File
		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return fmt.Errorf("mkdir all: %w", err)
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return fmt.Errorf("open file: %w", err)
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("open: %w", err)
		}

		_, err = io.Copy(outFile, rc)

		// Close the file without defer to close before next iteration of loop
		_ = outFile.Close()
		_ = rc.Close()

		if err != nil {
			return fmt.Errorf("copy: %w", err)
		}
	}
	return nil
}

func (t *GitHub) downloadFile(url string, file string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Add("Authorization", "token "+t.AccessToken)
	resp, err := t.Client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("read all: %w", err)
		}
		return errors.New(string(body))
	}

	out, err := os.Create(file)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("copy: %w", err)
	}

	return nil
}
