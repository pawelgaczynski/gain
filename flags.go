// Copyright (c) 2023 Paweł Gaczyński
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gain

const (
	acceptDataFlag uint64 = 1 << (uint64Size - 1 - iota)
	readDataFlag
	writeDataFlag
	addConnFlag
	closeConnFlag
)

const allFlagsMask = acceptDataFlag | readDataFlag | writeDataFlag | addConnFlag |
	closeConnFlag

func flagToString(flag uint64) string {
	switch {
	case flag&acceptDataFlag > 0:
		return "accept flag"
	case flag&readDataFlag > 0:
		return "read flag"
	case flag&writeDataFlag > 0:
		return "write flag"
	case flag&addConnFlag > 0:
		return "add conn flag"
	case flag&closeConnFlag > 0:
		return "close conn flag"
	}

	return "unknown flag"
}
