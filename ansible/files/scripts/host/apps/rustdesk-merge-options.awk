BEGIN {
  in_options = 0
  seen_options = 0
  seen_direct_server = 0
  seen_direct_access_port = 0
}

function emit_missing_options() {
  if (!seen_direct_server) {
    print "direct-server = \"Y\""
  }
  if (!seen_direct_access_port) {
    print "direct-access-port = \"21118\""
  }
}

/^\[options\][[:space:]]*$/ {
  seen_options = 1
  in_options = 1
  print
  next
}

/^\[/ {
  if (in_options) {
    emit_missing_options()
  }
  in_options = 0
  print
  next
}

in_options && /^[[:space:]]*direct-server[[:space:]]*=/ {
  print "direct-server = \"Y\""
  seen_direct_server = 1
  next
}

in_options && /^[[:space:]]*direct-access-port[[:space:]]*=/ {
  print "direct-access-port = \"21118\""
  seen_direct_access_port = 1
  next
}

{ print }

END {
  if (!seen_options) {
    if (NR > 0) {
      print ""
    }
    print "[options]"
    print "direct-server = \"Y\""
    print "direct-access-port = \"21118\""
  } else if (in_options) {
    emit_missing_options()
  }
}
