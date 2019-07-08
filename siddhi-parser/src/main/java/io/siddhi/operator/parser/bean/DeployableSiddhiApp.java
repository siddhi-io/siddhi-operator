package io.siddhi.operator.parser.bean;

import java.util.List;

public class DeployableSiddhiApp {
    private boolean persistenceEnabled = false;
    int replicas = 1;
    private String siddhiApp;

    public void setSourceDeploymentConfigs(List<SourceDeploymentConfig> sourceDeploymentConfigs) {
        this.sourceDeploymentConfigs = sourceDeploymentConfigs;
    }

    private List<SourceDeploymentConfig> sourceDeploymentConfigs = null;

    public DeployableSiddhiApp(String siddhiApp) {
        this.siddhiApp = siddhiApp;
    }

    public DeployableSiddhiApp(String siddhiApp, List<SourceDeploymentConfig> sourceList) {
        this.siddhiApp = siddhiApp;
        this.sourceDeploymentConfigs = sourceList;
    }

    public DeployableSiddhiApp (String siddhiApp, boolean persistenceEnabled){
        this.siddhiApp = siddhiApp;
        this.persistenceEnabled = persistenceEnabled;
    }
}
