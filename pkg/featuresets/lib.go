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

package featuresets

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"

	uiapi "kmodules.xyz/resource-metadata/apis/ui/v1alpha1"
)

//go:embed **/*.yaml
var fs embed.FS

func Render(f uiapi.Feature, uid int64) ([]byte, error) {
	filename := fmt.Sprintf("%s/%s.yaml", f.Spec.FeatureSet, f.Name)
	t, err := template.ParseFS(fs, filename)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	err = t.Execute(&buf, map[string]any{
		"uid": uid,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}
	return bytes.TrimSpace(buf.Bytes()), nil
}
