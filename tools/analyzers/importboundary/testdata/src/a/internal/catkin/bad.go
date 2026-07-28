package catkin

import (
	_ "a/internal/store" // want `internal/catkin must not import a/internal/store: catkin imports nothing from poplar`
)
