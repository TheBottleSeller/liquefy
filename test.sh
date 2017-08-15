#!/bin/sh
#
# All Your Tests Are Belong To Us
#

HOME=$(pwd)

function runTest {
  set -e
  PACKAGE=$1
  TEST=$2
  echo "Running test $TEST in package $PACKAGE"
  cd "$HOME/$PACKAGE"
  go test -v bargain/liquefy/$PACKAGE -run=$TEST
  if [ $? -ne 0 ]; then
    echo "$TEST in package $PACKAGE failed!"
  else
    echo "$TEST in package $PACKAGE succeeded!"
  fi
}

runTest "common" "TestCommon"
runTest "dockermanager" "TestDockerManager"
runTest "jobmanager" "TestJobManager"
runTest "workflow" "TestLocalWorkflow"
