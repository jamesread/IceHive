-- Runs once when the throwaway MySQL container initialises.
CREATE DATABASE IF NOT EXISTS icehive_sink;
CREATE USER IF NOT EXISTS 'icehive_sink'@'%' IDENTIFIED BY 'icehive-sink-demo';
GRANT ALL PRIVILEGES ON icehive_sink.* TO 'icehive_sink'@'%';
FLUSH PRIVILEGES;
