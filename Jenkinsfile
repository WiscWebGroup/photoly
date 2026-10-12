pipeline {
    agent any
    parameters {
        booleanParam(name: 'DEPLOY', defaultValue: false, description: 'Deploy the tested Flask backend')
        string(name: 'PHOTOLY_ENV_FILE', defaultValue: '.env', description: 'Production environment file on the Jenkins agent')
    }
    environment {
        COMPOSE_PROJECT_NAME = "photoly-ci-${BUILD_NUMBER}"
    }
    stages {
        stage('Build backend and test images') {
            steps { sh 'docker compose -f compose.test.yml build tests backend smoke' }
        }
        stage('API contracts and database integration') {
            steps { sh 'docker compose -f compose.test.yml run --rm tests' }
        }
        stage('HTTP workflow and graceful shutdown') {
            steps { sh 'docker compose -f compose.test.yml run --rm smoke' }
        }
        stage('Deploy') {
            when { expression { params.DEPLOY } }
            steps {
                sh 'docker compose --env-file "$PHOTOLY_ENV_FILE" -p photoly up -d --build'
            }
        }
    }
    post {
        always { sh 'docker compose -f compose.test.yml down -v --remove-orphans' }
    }
}
