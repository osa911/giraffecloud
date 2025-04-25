# Release management
.PHONY: release-dry-run release snapshot

release-dry-run: ## Test the release process without publishing
	goreleaser release --snapshot --clean --skip-publish

release: ## Create and publish a new release
	goreleaser release --clean

snapshot: ## Create a snapshot release for testing
	goreleaser release --snapshot --skip-publish --clean