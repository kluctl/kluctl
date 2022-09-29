/*
Copyright © 2022 Alexander Block <ablock84@gmail.com>

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
package main

import (
	"github.com/kluctl/kluctl/v2/cmd/kluctl/commands"
	version2 "github.com/kluctl/kluctl/v2/pkg/version"
)

var version = "0.0.0"

func main() {
	version2.SetVersion(version)
	commands.Execute()
}
