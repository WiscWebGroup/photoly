pipeline {
    agent any

    stages {
        stage('Initialize') {
            steps {
                echo 'Initializing....'
    ***REMOVED***
***REMOVED***

        stage('Stop Old Backend') {
            steps {
                sh 'fuser -k -n tcp 8088 || true'
    ***REMOVED***
***REMOVED***

        stage('Deploy New Backend') {
            steps {
                sh 'cp /home/ubuntu/photoly_b/config/application.properties /root/.jenkins/workspace/photoly/photoly_backend/src/main/resources/application.properties'
                sh '''cd photoly_backend
mvn clean package -Dmaven.test.skip=true'''
                sh 'mv -f /root/.jenkins/workspace/photoly/photoly_backend/target/photoly.war /home/ubuntu/photoly_b/photoly.war'
                sh 'chmod 777 /home/ubuntu/photoly_b/photoly.war'
                sh 'JENKINS_NODE_COOKIE=dontKillMe nohup java -jar /home/ubuntu/photoly_b/photoly.war > /home/ubuntu/photoly_b/photoly.log 2>1&'
    ***REMOVED***
***REMOVED***

        stage('Finish') {
            steps {
                echo 'Finished.'
    ***REMOVED***
***REMOVED***
        
        stage('Initialize Frontend Deployment') {
            steps {
                sh 'rm -rf /home/ubuntu/photoly_f/photoly_frontend'
                sh 'cp -r /root/.jenkins/workspace/photoly/photoly_frontend /home/ubuntu/photoly_f'
                sh 'rm /home/ubuntu/photoly_f/photoly_frontend/next.config.js'
                sh 'cp /home/ubuntu/photoly_f/config/next.config.js /home/ubuntu/photoly_f/photoly_frontend'
    ***REMOVED***
***REMOVED***
        
        stage('Build Frontend and Deploy') {
            steps {
                sh 'cd /home/ubuntu/photoly_f/photoly_frontend'
                sh 'ls'
                sh 'npm install'
                sh 'npm run build'
                sh 'JENKINS_NODE_COOKIE=dontKillMe nohup npm run start -- -p 8084 > /home/ubuntu/photoly_f/photoly_f.log 2>1&'
    ***REMOVED***
***REMOVED***
        
***REMOVED***
***REMOVED***
