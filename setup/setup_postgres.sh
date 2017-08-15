DBNAME=liquiddev
DBUSER=liquiddev
psql postgres -c "DROP USER $DBUSER"
psql postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = 'liquiddev';"
psql postgres -c "DROP DATABASE $DBNAME"
psql postgres -c "DROP USER $DBUSER"
psql postgres -c "CREATE DATABASE $DBNAME"
psql liquefydb -c "CREATE USER $DBUSER"
