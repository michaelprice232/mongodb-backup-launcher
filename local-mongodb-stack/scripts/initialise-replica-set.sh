#!/bin/bash

set -e

USERNAME="root"
PASSWORD="example"

mongo --host localhost --port 27017 -u $USERNAME -p $PASSWORD --eval '
rs.initiate({
  _id: "rs0",
  members: [
    { _id: 0, host: "mongo1:27017", "priority": 2 },
    { _id: 1, host: "mongo2:27017", "priority": 2 },
    { _id: 2, host: "mongo3:27017", "priority": 0.5 }
  ]
})
'
