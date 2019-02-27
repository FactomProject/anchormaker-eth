go install -ldflags "-X github.com/FactomProject/anchormaker-eth/api.Build=`git rev-parse HEAD` -X github.com/FactomProject/anchormaker-eth/api.Version=`cat VERSION`"
