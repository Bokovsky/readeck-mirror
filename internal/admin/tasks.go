// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package admin

import (
	"encoding/json"
	"log/slog"

	"github.com/doug-martin/goqu/v9"

	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/bus"
	"codeberg.org/readeck/readeck/pkg/superbus"
)

var deleteUserTask superbus.Task

func init() {
	bus.OnReady(func() {
		deleteUserTask = bus.Tasks().NewTask(
			"user.delete",
			superbus.WithTaskDelay(20),
			superbus.WithUnmarshall(func(data []byte) any {
				var res int
				err := json.Unmarshal(data, &res)
				if err != nil {
					panic(err)
				}
				return res
			}),
			superbus.WithTaskHandler(deleteUserHandler),
		)
	})
}

func deleteUserHandler(data any) {
	id := data.(int)
	logger := slog.With(slog.Int("id", id))

	u, err := users.Users.GetOne(goqu.C("id").Eq(id))
	if err != nil {
		logger.Error("user retrieve", slog.Any("err", err))
		return
	}

	if err := deleteUser(u); err != nil {
		logger.Error("user removal", slog.Any("err", err))
		return
	}

	logger.Info("user removed")
}
