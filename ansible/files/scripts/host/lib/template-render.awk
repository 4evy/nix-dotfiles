function escape_replacement(value) {
  gsub(/\\/, "\\\\", value)
  gsub(/&/, "\\\\&", value)
  return value
}

BEGIN {
  for (i = 2; i < ARGC; i += 2) {
    replacements["@" ARGV[i] "@"] = escape_replacement(ARGV[i + 1])
  }
  ARGC = 2
}

{
  for (placeholder in replacements) {
    gsub(placeholder, replacements[placeholder])
  }
  print
}
