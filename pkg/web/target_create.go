package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	uuid "github.com/satori/go.uuid"
	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/gh"
	"github.com/whywaita/myshoes/pkg/logger"
)

func handleTargetCreate(w http.ResponseWriter, r *http.Request, ds datastore.Datastore) {
	// input values: scope, gpt, ghe_domain, resource_type
	ctx := r.Context()
	inputTarget := targetCreateParam{}
	if err := json.NewDecoder(r.Body).Decode(&inputTarget); err != nil {
		logger.Logf(false, "failed to decode request body: %+v", err)
		outputErrorMsg(w, http.StatusBadRequest, "json decode error")
		return
	}

	if ok, err := isValidTargetCreateParam(inputTarget); !ok {
		logger.Logf(false, "failed to validate input: %+v", err)
		outputErrorMsg(w, http.StatusBadRequest, err.Error())
		return
	}
	t := inputTarget.toDS()
	if err := isValidScopeAndToken(ctx, t.Scope, t.GHEDomain.String, t.GitHubPersonalToken, t.GHEDomain.Valid); err != nil {
		outputErrorMsg(w, http.StatusBadRequest, err.Error())
		return
	}

	target, err := ds.GetTargetByScope(ctx, t.GHEDomain.String, t.Scope)
	var targetUUID uuid.UUID

	switch {
	case errors.Is(err, datastore.ErrNotFound):
		// not created, will be creating
		u, err := createNewTarget(ctx, t, ds)
		if err != nil {
			outputErrorMsg(w, http.StatusInternalServerError, err.Error())
			return
		}
		targetUUID = *u
	case err != nil:
		logger.Logf(false, "failed to get target by scope [ghe_domain: %s scope: %s]: %+v", t.GHEDomain.String, t.Scope, err)
		outputErrorMsg(w, http.StatusInternalServerError, "datastore error")
		return

	case target.Status != datastore.TargetStatusDeleted:
		// already registered
		errMsg := fmt.Sprintf("%s is already registered, current status is %s.", t.Scope, target.Status)
		outputErrorMsg(w, http.StatusBadRequest, errMsg)
		return
	case target.Status == datastore.TargetStatusDeleted:
		// deleted, need to recreate
		// TODO: need to update resource
		if err := ds.UpdateTargetStatus(ctx, target.UUID, datastore.TargetStatusActive, ""); err != nil {
			logger.Logf(false, "failed to recreate target: %+v", err)
			outputErrorMsg(w, http.StatusInternalServerError, "datastore recreate error")
			return
		}
		targetUUID = target.UUID
	}

	createdTarget, err := ds.GetTarget(ctx, targetUUID)
	if err != nil {
		logger.Logf(false, "failed to get recently target in datastore: %+v", err)
		outputErrorMsg(w, http.StatusInternalServerError, "datastore get error")
		return
	}
	sanitized := sanitizeTarget(createdTarget)

	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sanitized)
	return
}

func isValidScopeAndToken(ctx context.Context, scope, gheDomain, githubPersonalToken string, gheDomainValid bool) error {
	if err := GHExistGitHubRepositoryFunc(scope, gheDomain, gheDomainValid, githubPersonalToken); err != nil {
		logger.Logf(false, "failed to found github repository: %+v", err)
		return fmt.Errorf("github scope is invalid (maybe, repository is not found)")
	}

	client, err := gh.NewClient(ctx, githubPersonalToken, gheDomain)
	if err != nil {
		logger.Logf(false, "failed to create GitHub client: %+v", err)
		return fmt.Errorf("invalid github token in input scope")
	}
	owner, repo := gh.DivideScope(scope)
	if _, err := GHListRunnersFunc(ctx, client, owner, repo); err != nil {
		logger.Logf(false, "failed to get list of registered runners: %+v", err)
		return fmt.Errorf("failed to get list of registered runners (maybe, invalid scope or token?)")
	}

	return nil
}

func createNewTarget(ctx context.Context, input datastore.Target, ds datastore.Datastore) (*uuid.UUID, error) {
	input.UUID = uuid.NewV4()
	now := time.Now().UTC()
	input.CreatedAt = now
	input.UpdatedAt = now
	if err := ds.CreateTarget(ctx, input); err != nil {
		logger.Logf(false, "failed to create target in datastore: %+v", err)
		return nil, fmt.Errorf("datastore create error")
	}

	return &input.UUID, nil
}
