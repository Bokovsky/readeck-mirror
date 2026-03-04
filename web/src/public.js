// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Application} from "@hotwired/stimulus"
import VideoController from "./controllers/video_controller"
import ThemeController from "./controllers/theme_controller.js"

const application = Application.start()
application.register("video", VideoController)
application.register("theme", ThemeController)
