function refreshWithoutQueryString() {
    if (window.location.href.split("?").length > 1) {
        window.location = window.location.pathname
    } else {
        location.reload()
    }
}

function updateLanguage(path) {
    var complete = false
    var xmlHttp = new XMLHttpRequest()

    function finish() {
        if (!complete) {
            complete = true
            refreshWithoutQueryString()
        }
    }

    xmlHttp.onreadystatechange = function() {
        if (xmlHttp.readyState === 4) {
            finish()
        }
    }

    xmlHttp.onerror = function() {
        finish()
    }

    xmlHttp.open("POST", path, true)
    xmlHttp.send(null)
}

function toggleEnglish() {
    updateLanguage("/language/english")
}

function toggleWelsh() {
    updateLanguage("/language/welsh")
}
