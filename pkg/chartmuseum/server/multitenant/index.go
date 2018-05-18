package multitenant

import (
	pathutil "path"

	cm_logger "github.com/xunchangguo/chartmuseum/pkg/chartmuseum/logger"
	cm_repo "github.com/xunchangguo/chartmuseum/pkg/repo"
	cm_storage "github.com/xunchangguo/chartmuseum/pkg/storage"
)

var (
	indexFileContentType = "application/x-yaml"
)

func (server *MultiTenantServer) getIndexFile(log cm_logger.LoggingFn, repo string) (*cm_repo.Index, *HTTPError) {
	server.initCachedIndexFile(log, repo)

	fo := <-server.getChartList(log, repo)

	if fo.err != nil {
		errStr := fo.err.Error()
		log(cm_logger.ErrorLevel, errStr,
			"repo", repo,
		)
		return nil, &HTTPError{500, errStr}
	}

	objects := server.getRepoObjectSlice(repo)
	diff := cm_storage.GetObjectSliceDiff(objects, fo.objects)

	// return fast if no changes
	if !diff.Change {
		return server.IndexCache[repo].RepositoryIndex, nil
	}

	ir := <-server.regenerateRepositoryIndex(log, repo, diff)

	if ir.err != nil {
		errStr := ir.err.Error()
		log(cm_logger.ErrorLevel, errStr,
			"repo", repo,
		)
		return ir.index, &HTTPError{500, errStr}
	}

	if server.UseStatefiles {
		// Dont wait, save index-cache.yaml to storage in the background.
		// It is not crucial if this does not succeed, we will just log any errors
		go server.saveStatefile(log, repo, ir.index.Raw)
	}

	return ir.index, nil
}

func (server *MultiTenantServer) saveStatefile(log cm_logger.LoggingFn, repo string, content []byte) {
	err := server.StorageBackend.PutObject(pathutil.Join(repo, cm_repo.StatefileFilename), content)
	if err != nil {
		log(cm_logger.WarnLevel, "Error saving index-cache.yaml",
			"repo", repo,
			"error", err.Error(),
		)
	}
	log(cm_logger.DebugLevel, "index-cache.yaml saved in storage",
		"repo", repo,
	)
}

func (server *MultiTenantServer) getRepoObjectSlice(repo string) []cm_storage.Object {
	var objects []cm_storage.Object
	for _, entry := range server.IndexCache[repo].RepositoryIndex.Entries {
		for _, chartVersion := range entry {
			object := cm_repo.StorageObjectFromChartVersion(chartVersion)
			objects = append(objects, object)
		}
	}
	return objects
}
