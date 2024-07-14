setup_file() {
    STEAMPIPE_CACHE=false STEAMPIPE_LOG_LEVEL=DEBUG steampipe service start || true # So tests run faster, since they won't need to run the entire
    sleep 2 # Give some time for Steampipe to load plugins
}
teardown_file() {
    steampipe service stop
}

setup() {
    load 'test_helper/bats-support/load'
    load 'test_helper/bats-assert/load'
}


@test "Steampipe is installed" {
    run steampipe --version
    assert_success
    assert_output --regexp "^Steampipe v[0-9]+\.[0-9]+\.[0-9]+$"
}

@test "MongoDB plugin is installed" {
  run steampipe query "select * from steampipe_connection WHERE name='mongodb'"
  assert_success
  assert_output --partial "ready"
}

@test "Reads count of records from accounts collection" {
  run steampipe query "select count(*) from mongodb.accounts"
  assert_success
  assert_output - <<- 'EOF'
+-------+
| count |
+-------+
| 1746  |
+-------+
EOF
}

@test "Applies simple string filter" {
  run steampipe query "select * from mongodb.customers where username='gregoryharrison'"
  assert_success
  assert_output --partial "Natalie Ford" # $.name of that customer
}

@test "Applies regex string filter w/ case insensitivity" {
  run steampipe query "select * from mongodb.customers where username ~* '^Anthony'"
  assert_success
  assert_output --partial "anthonyandrade"
  assert_output --partial "anthonygarza"
  assert_output --partial "anthony45"
}

@test "Applies numeric filter" {
  run steampipe query "select account_id from mongodb.accounts where \"limit\"<=3000"
  assert_success
  assert_output --partial "417993" # one of two account IDs
  assert_output --partial "113123"
}

@test "Applies JSONB Exists All operator" {
  # Accounts with BOTH InvestmentStock and Brokerage
  run steampipe query "select account_id from mongodb.accounts where products ?& array['InvestmentStock', 'Brokerage']"
  assert_success
  assert_output --partial "557378" # this one has both of the expected products
  refute_output --partial "278603" # this one has Commodity and InvestmentStock but not Brokerage
}

