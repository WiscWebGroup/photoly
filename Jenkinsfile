pipeline {
    agent any

    stages {
        stage('Initialize') {
            steps {
                echo 'Initializing....'
            }
        }

        stage('Stop Old Backend & Frontend') {
            steps {
                sh 'fuser -k -n tcp 8088 || true'
            }
        }

        stage('Deploy New Backend') {
            steps {
                sh 'cp /home/ubuntu/photoly_b/config/application.properties /root/.jenkins/workspace/photoly/photoly_backend/src/main/resources/application.properties'
                sh '''cd photoly_backend
mvn clean package -Dmaven.test.skip=true'''
                sh 'mv -f /root/.jenkins/workspace/photoly/photoly_backend/target/photoly.war /home/ubuntu/photoly_b/photoly.war'
                sh 'chmod 777 /home/ubuntu/photoly_b/photoly.war'
                sh 'JENKINS_NODE_COOKIE=dontKillMe nohup java -jar /home/ubuntu/photoly_b/photoly.war > /home/ubuntu/photoly_b/photoly.log 2>1&'
            }
        }
        
        stage('Initialize Frontend Deployment') {
            steps {
                sh 'rm -rf /home/ubuntu/photoly_f/photoly_frontend'
                sh 'cp -r /root/.jenkins/workspace/photoly/photoly_frontend_vue /home/ubuntu/photoly_f'
                sh 'rm /home/ubuntu/photoly_f/photoly_frontend_vue/.env'
                sh 'cp /home/ubuntu/photoly_f/config/.env /home/ubuntu/photoly_f/photoly_frontend_vue'
            }
        }
        
        stage('Build Frontend and Deploy') {
            steps {
                dir('/home/ubuntu/photoly_f/photoly_frontend_vue') {
                    sh 'npm install'
                    sh 'npm run build'
                }
            }
        }
        
        stage('Finish') {
            steps {
                echo 'Finished.'
            }
        }
        
    }
}
