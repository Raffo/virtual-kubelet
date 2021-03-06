package container

import (
	"net/http"

	"github.com/hyperhq/hypercli/api/server/httputils"
	"golang.org/x/net/context"
)

// getContainersByName inspects containers configuration and serializes it as json.
func (s *containerRouter) getContainersByName(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	displaySize := httputils.BoolValue(r, "size")

	version := httputils.VersionFromContext(ctx)
	json, err := s.backend.ContainerInspect(vars["name"], displaySize, version)
	if err != nil {
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, json)
}
