/*
Copyright 2015 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"k8s.io/contrib/mungegithub/pulls/jenkins"

	"github.com/golang/glog"
)

// E2ETester is the object which will contact a jenkins instance and get
// information about recent jobs
type E2ETester struct {
	JenkinsHost string
	JenkinsJobs []string

	sync.Mutex
	BuildStatus map[string]string // protect by mutex
}

func (e *E2ETester) locked(f func()) {
	e.Lock()
	defer e.Unlock()
	f()
}

func (e *E2ETester) setBuildStatus(build, status string) {
	e.Lock()
	defer e.Unlock()
	e.BuildStatus[build] = status
}

// Stable is called to make sure all of the jenkins jobs are stable
func (e *E2ETester) Stable() bool {
	// Test if the build is stable in Jenkins
	jenkinsClient := &jenkins.JenkinsClient{Host: e.JenkinsHost}

	allStable := true
	for _, build := range e.JenkinsJobs {
		glog.V(2).Infof("Checking build stability for %s", build)
		stable, err := jenkinsClient.IsBuildStable(build)
		if err != nil {
			glog.Errorf("Error checking build %v : %v", build, err)
			e.setBuildStatus(build, "Error checking: "+err.Error())
			allStable = false
			continue
		}
		if stable {
			e.setBuildStatus(build, "Stable")
		} else {
			e.setBuildStatus(build, "Not Stable")
			allStable = false
		}
	}
	return allStable
}

func (e *E2ETester) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	var (
		data []byte
		err  error
	)
	e.locked(func() {
		data, err = json.MarshalIndent(e.BuildStatus, "", "\t")
	})

	if err != nil {
		glog.Errorf("Failed to encode status: %#v %v", e.BuildStatus, err)
		res.Header().Set("Content-type", "text/plain")
		res.WriteHeader(http.StatusInternalServerError)
		res.Write([]byte(err.Error()))
		res.Write([]byte(fmt.Sprintf("%#v", e.BuildStatus)))
	} else {
		res.Header().Set("Content-type", "application/json")
		res.WriteHeader(http.StatusOK)
		res.Write(data)
	}
}
