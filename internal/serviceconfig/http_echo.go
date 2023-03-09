/*
 * Copyright 2021 OpsMx, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License")
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package serviceconfig

import (
	"context"

	pb "github.com/skandragon/grpc-datacon/internal/tunnel"
)

type HTTPEcho interface {
	// Headers is called once to send the appropriate headers.
	Headers(ctx context.Context, h *pb.TunnelHeaders) error
	// Data is called one or more times to send data.
	Data(ctx context.Context, d *pb.Data) error
	// Fail can be called to indicate no more calls will be made.  This may happen
	// without calling Headers() or Data(), or after calling one or both.  If
	// headers have been sent, this should send a EOF Data frame.
	Fail(ctx context.Context, httpCode int, err error) error
	// Done indicates the session ended.  If headers have not been sent,
	// this is an error.  Data may not be called, and Done should send
	// an EOF Data frame.
	Done(ctx context.Context)
}
