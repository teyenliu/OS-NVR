// Copyright 2020-2022 The OS-NVR Authors.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package minio

import (
	"fmt"
	"nvr"
	"os"
	"strings"
)

func init() {
	nvr.RegisterTplHook(modifyTemplates)
}

func modifyTemplates(pageFiles map[string]string) error {
	js, exists := pageFiles["settings.js"]
	if !exists {
		return fmt.Errorf("minio: settings.js: %w", os.ErrNotExist)
	}
	pageFiles["settings.js"] = modifySettingsjs(js)
	return nil
}

func modifySettingsjs(tpl string) string { //nolint:funlen
	const target = "logLevel: fieldTemplate.select("

	const javascript = `
	minio: (() => {
		const fields = {
			enable: fieldTemplate.toggle("Enable", "true"),
			deletelocal: fieldTemplate.toggle("Del local", "false"),
		};
		const form = newForm(fields);
		const modal = newModal("Minio", form.html());

		let value = {};

		let isRendered = false;
		const render = (element) => {
			if (isRendered) {
				return;
			}
			element.insertAdjacentHTML("beforeend", modal.html)
			element.querySelector(".js-modal").style.maxWidth = "12rem";

			const $modalContent = modal.init(element)
			form.init($modalContent);

			modal.onClose(() => {
				// Get value.
				for (const key of Object.keys(form.fields)) {
					value[key] = form.fields[key].value();
				}
			});
			isRendered = true;
		}

		const update = () => {
			// Set value.
			for (const key of Object.keys(form.fields)) {
				if (form.fields[key] && form.fields[key].set) {
					if (value[key]) {
						form.fields[key].set(value[key]);
					} else {
						form.fields[key].set("");
					}
				}
			}
		}

		const id = uniqueID()

		return {
			html: ` + "`" + `
				<li id="${id}" class="form-field" style="display:flex;">
					<label class="form-field-label">Minio</label>
					<div>
						<button class="form-field-edit-btn" style="background: var(--color3);">
							<img src="static/icons/feather/edit-3.svg"/>
						</button>
					</div>
				</li> ` + "`" + `,
			value() {
				return JSON.stringify(value);
			},
			set(input) {
				if (input) {
					value = JSON.parse(input);
				} else {
					value = {};
				}
			},
			validate() {
				if (!isRendered) {
					return "";
				}
				const err = form.validate()
				if (err != "") {
					return "Minio: " + err;
				}
				return "";
			},
			init($parent) {
				const element = $parent.querySelector("#"+id)
				element.querySelector(".form-field-edit-btn").addEventListener("click", () => {
					render(element)
					update()
					modal.open()
				});
			},
		}
	})(),`

	return strings.ReplaceAll(tpl, target, javascript+target)
}
