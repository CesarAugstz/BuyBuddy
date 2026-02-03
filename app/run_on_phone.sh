#!/bin/bash
export JAVA_HOME=/usr/lib/jvm/java-21-openjdk
export PATH=$JAVA_HOME/bin:$PATH
export GRADLE_OPTS="-Dorg.gradle.java.home=$JAVA_HOME"
cd /home/tyler/dev/projects/easybuy/app
flutter run -d SWNZ6PGIAMDYYHX4
