// Package present consumes rendered frames and presents them. A Presenter pulls
// frames from a FrameFunc, which renders the frame at a given animation time and
// must be deterministic in that time, so the rendered motion is reproducible.
//
// It ships a single backend: an animated-GIF writer built on the standard
// library alone (zero third-party dependencies). The Presenter interface is the
// seam for additional output backends, which can be added without the renderer
// core depending on them.
package present
