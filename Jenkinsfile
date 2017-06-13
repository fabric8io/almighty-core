#!/usr/bin/groovy
@Library('github.com/fabric8io/fabric8-pipeline-library@master')
def dummy
goTemplate{
  dockerNode{
    goRelease{
      githubOrganisation = 'fabric8io'
      dockerOrganisation = 'fabric8'
      project = 'almighty-core'
      dockerBuildOptions = '--file Dockerfile.deploy'
    }
  }
}
