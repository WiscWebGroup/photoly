pipeline {
    agent any

    stages {
        stage('Initialize') {
            steps {
                echo 'Initializing...'
    ***REMOVED***
***REMOVED***

        stage('Stop Old Backend') {
            steps {
                sh 'fuser -k -n tcp 8088'
    ***REMOVED***
***REMOVED***

        stage('Deploy New Backend') {
            steps {
                sh 'mvn clean package -Dmaven.test.skip=true'
                sh 'mv -f /root/.jenkins/workspace/photoly/api/target/photoly.war /home/ubuntu/photoly_b/photoly.war'
                sh 'chmod 777 /home/ubuntu/photoly_b/photoly.war'
                sh 'nohup java -jar /home/ubuntu/photoly_b/photoly.war > /home/ubuntu/photoly_b/photoly.log 2>1&'
    ***REMOVED***
***REMOVED***

        stage('Finish') {
            steps {
                echo 'Finished.'
    ***REMOVED***
***REMOVED***
***REMOVED***
***REMOVED***
