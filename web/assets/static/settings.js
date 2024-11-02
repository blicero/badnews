// Time-stamp: <2024-10-31 20:19:54 krylon>
// -*- mode: javascript; coding: utf-8; -*-
// Copyright 2020 Benjamin Walkenhorst <krylon@gmx.net>

"use strict";

var settings = {
    "beacon": {
        "active": false,
        "interval": 1000,
    },

    "messages": {
        "queryEnabled": true,
        "interval": 5000,
        "maxShow": 25,
    },

    "news": {
        "hideBoring": false,
    }
};

function initSettings() {
    var item;
    
    settings.beacon.active =
        JSON.parse(localStorage.getItem("beacon.active")) ? true : false;
    item = JSON.parse(localStorage.getItem("beacon.interval"));
    if (Number.isInteger(item)) {
        settings.beacon.interval = item;
    }

    settings.messages.queryEnabled =
        JSON.parse(localStorage.getItem("messages.queryEnabled"));

    if (null == settings.messages.queryEnabled) {
        settings.messages.queryEnabled = false;
        localStorage.setItem("messages.queryEnabled", false);
    }

    item = JSON.parse(localStorage.getItem("messages.interval"));
    if (Number.isInteger(item)) {
        settings.messages.interval = item;
    }

    item = JSON.parse(localStorage.getItem("messages.maxShow"));
    if (Number.isInteger(item)) {
        settings.messages.maxShow = item;
    }

    item = JSON.parse(localStorage.getItem("news.hideBoring"))
    if (typeof(item) == "boolean") {
        settings.news.hideBoring = item
    }
} // function initSettings()

function saveSetting(category, attribute, newValue) {
    var cat = settings[category];
    if (cat == undefined) {
        console.log("Invalid category: " + category);
        return;
    }

    var att = cat[attribute];
    if (att == undefined) {
        console.log("Invalid attribute: " + attribute);
        return;
    }

    var key = category + "." + attribute;
    localStorage.setItem(key, newValue);
    settings[category][attribute] = newValue;
} // function saveSetting(group, member, newValue)

