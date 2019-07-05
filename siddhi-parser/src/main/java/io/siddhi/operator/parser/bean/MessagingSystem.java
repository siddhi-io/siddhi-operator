package io.siddhi.operator.parser.bean;

import com.google.gson.annotations.SerializedName;

public class MessagingSystem {

    @SerializedName("type")
    private String type;

    @SerializedName("config")
    private MessagingConfig config;

    public String getType() {
        return type;
    }

    public void setType(String type) {
        this.type = type;
    }

    public MessagingConfig getConfig() {
        return config;
    }

    public void setConfig(MessagingConfig config) {
        this.config = config;
    }
}
