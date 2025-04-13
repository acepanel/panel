package api

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type APITestSuite struct {
	suite.Suite
	api *API
}

func TestAPITestSuite(t *testing.T) {
	suite.Run(t, &APITestSuite{
		api: NewAPI("2.3.0", "en"),
	})
}

func (s *APITestSuite) TestGetLatestVersion() {
	_, err := s.api.LatestVersion()
	s.NoError(err)
}

func (s *APITestSuite) TestGetIntermediateVersions() {
	_, err := s.api.IntermediateVersions()
	s.NoError(err)
}

func (s *APITestSuite) TestGetApps() {
	_, err := s.api.Apps()
	s.NoError(err)
}

func (s *APITestSuite) TestGetAppBySlug() {
	_, err := s.api.AppBySlug("nginx")
	s.NoError(err)
}

func (s *APITestSuite) TestGetRewritesByType() {
	_, err := s.api.RewritesByType("nginx")
	s.NoError(err)
}
