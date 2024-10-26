/*
Copyright AppsCode Inc. and Contributors

Licensed under the AppsCode Community License 1.0.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://github.com/appscode/licenses/raw/1.0.0/AppsCode-Community-1.0.0.md

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tracker

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	core "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	OpenShiftClusterClaim = "openshift.ace.info"
	KeyUid                = "openshift.io/sa.scc.uid-range"
	KeyFsGroup            = "openshift.io/sa.scc.supplemental-groups"
	UidRange              = 10000
	UidNone               = -1
)

func GetUid(kc client.Reader, ns string) (int64, int64, error) {
	var obj core.Namespace
	err := kc.Get(context.TODO(), client.ObjectKey{Name: ns}, &obj)
	if err != nil {
		return UidNone, UidNone, client.IgnoreNotFound(err)
	}

	curUid, foundUid := obj.Annotations[KeyUid]
	curFsGroupUid, foundFsGroup := obj.Annotations[KeyFsGroup]
	if !foundUid && !foundFsGroup {
		return UidNone, UidNone, nil
	}
	if curUid != curFsGroupUid {
		return UidNone, UidNone, fmt.Errorf("runAsUser %s and fsGroup %s uid range does not match", curUid, curFsGroupUid)
	}

	strUid, strRange, ok := strings.Cut(curUid, "/")
	if !ok {
		return UidNone, UidNone, fmt.Errorf("%s annotation value is not in <start>/<range> format", KeyUid)
	}

	uid, err := strconv.ParseInt(strUid, 10, 64)
	if err != nil {
		return UidNone, UidNone, fmt.Errorf("%s annotation start uid is not an interger", KeyUid)
	}
	uidRange, err := strconv.ParseInt(strRange, 10, 64)
	if err != nil {
		return UidNone, UidNone, fmt.Errorf("%s annotation range is not an interger", KeyUid)
	}
	return uid, uidRange, nil
}
