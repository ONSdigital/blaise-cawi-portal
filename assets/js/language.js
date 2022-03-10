function toggleEnglish() {
    var xmlHttp = new XMLHttpRequest
    xmlHttp.open("GET", "/language/english", false);
    xmlHttp.send(null);
    if (window.location.href.split("?").length > 1) {
        window.location = window.location.pathname
    } else {
        location.reload()
    }
}

function toggleWelsh() {
    var xmlHttp = new XMLHttpRequest
    xmlHttp.open("GET", "/language/welsh", false);
    xmlHttp.send(null);
    if (window.location.href.split("?").length > 1) {
        window.location = window.location.pathname
    } else {
        location.reload()
    }
}
