/*
 * Copyright (c) 2017-2018 Miguel Ángel Ortuño.
 * See the COPYING file for more information.
 */

package compress

import "io"

type Compressor interface {
	io.ReadWriter
}
