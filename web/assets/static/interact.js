// Time-stamp: <2024-10-18 22:18:40 krylon>
// -*- mode: javascript; coding: utf-8; -*-
// Copyright 2015-2020 Benjamin Walkenhorst <krylon@gmx.net>
//
// This file has grown quite a bit larger than I had anticipated.
// It is not a /big/ problem right now, but in the long run, I will have to
// break this thing up into several smaller files.

'use strict'

function defined (x) {
    return undefined !== x && null !== x
}

function fmtDateNumber (n) {
    return (n < 10 ? '0' : '') + n.toString()
} // function fmtDateNumber(n)

function timeStampString (t) {
    if ((typeof t) === 'string') {
        return t
    }

    const year = t.getYear() + 1900
    const month = fmtDateNumber(t.getMonth() + 1)
    const day = fmtDateNumber(t.getDate())
    const hour = fmtDateNumber(t.getHours())
    const minute = fmtDateNumber(t.getMinutes())
    const second = fmtDateNumber(t.getSeconds())

    const s =
          year + '-' + month + '-' + day +
          ' ' + hour + ':' + minute + ':' + second
    return s
} // function timeStampString(t)

function fmtDuration (seconds) {
    let minutes = 0
    let hours = 0

    while (seconds > 3599) {
        hours++
        seconds -= 3600
    }

    while (seconds > 59) {
        minutes++
        seconds -= 60
    }

    if (hours > 0) {
        return `${hours}h${minutes}m${seconds}s`
    } else if (minutes > 0) {
        return `${minutes}m${seconds}s`
    } else {
        return `${seconds}s`
    }
} // function fmtDuration(seconds)

function beaconLoop () {
    try {
        if (settings.beacon.active) {
            const req = $.get('/ajax/beacon',
                              {},
                              function (response) {
                                  let status = ''

                                  if (response.Status) {
                                      status = 
                                          response.Message +
                                          ' running on ' +
                                          response.Hostname +
                                          ' is alive at ' +
                                          response.Timestamp
                                  } else {
                                      status = 'Server is not responding'
                                  }

                                  const beaconDiv = $('#beacon')[0]

                                  if (defined(beaconDiv)) {
                                      beaconDiv.innerHTML = status
                                      beaconDiv.classList.remove('error')
                                  } else {
                                      console.log('Beacon field was not found')
                                  }
                              },
                              'json'
                             ).fail(function () {
                                 const beaconDiv = $('#beacon')[0]
                                 beaconDiv.innerHTML = 'Server is not responding'
                                 beaconDiv.classList.add('error')
                                 // logMsg("ERROR", "Server is not responding");
                             })
        }
    } finally {
        window.setTimeout(beaconLoop, settings.beacon.interval)
    }
} // function beaconLoop()

function beaconToggle () {
    settings.beacon.active = !settings.beacon.active
    saveSetting('beacon', 'active', settings.beacon.active)

    if (!settings.beacon.active) {
        const beaconDiv = $('#beacon')[0]
        beaconDiv.innerHTML = 'Beacon is suspended'
        beaconDiv.classList.remove('error')
    }
} // function beaconToggle()

/*
  The ‘content’ attribute of Window objects is deprecated.  Please use ‘window.top’ instead. interact.js:125:8
  Ignoring get or set of property that has [LenientThis] because the “this” object is incorrect. interact.js:125:8

*/

function db_maintenance () {
    const maintURL = '/ajax/db_maint'

    const req = $.get(
        maintURL,
        {},
        function (res) {
            if (!res.Status) {
                console.log(res.Message)
                postMessage(new Date(), 'ERROR', res.Message)
            } else {
                const msg = 'Database Maintenance performed without errors'
                console.log(msg)
                postMessage(new Date(), 'INFO', msg)
            }
        },
        'json'
    ).fail(function () {
        const msg = 'Error performing DB maintenance'
        console.log(msg)
        postMessage(new Date(), 'ERROR', msg)
    })
} // function db_maintenance()

function toggle_visibility(hostID) {
    const query = `tr.Host${hostID}`
    const visible = !$(`#show_${hostID}`)[0].checked
    $(query).each(function () {
        if (visible) {
            $(this).hide()
        } else {
            $(this).show()
        }
    })
} // function toggle_visibility(hostID)

function filter_source(src) {
    const query = `tr.src_${src}`
    const visible = !$(`#filter_src_${src}`)[0].checked
    $(query).each(function () {
        if (visible) {
            $(this).hide()
        } else {
            $(this).show()
        }
    })
} // function filter_source(src)

function toggle_visibility_all() {
    const visible = !$("#filter_toggle_all")[0].checked

    const boxes = jQuery("#sources_list input.filter_src_check")

    for (let i = 0; i < boxes.length; i++) {
        boxes[i].checked = !visible
    }

    const rows = jQuery("#records > tr")

    for (let i = 0; i < rows.length; i++) {
        rows[i].hide()
    }
} // function toggle_visibility_all()

function search_load_results(sid, page) {
    const addr = `/ajax/search/load/${sid}/${page}`

    const reply = $.get(addr,
                        {},
                        function (res) {
                            if (res.Status) {
                                jQuery("#results")[0].innerHTML = res.Payload["results"]

                                const params = JSON.parse(res.Payload["search"])

                                for (var h of params.Query.hosts) {
                                    const filter_id = `#filter_host_${h}`
                                    jQuery(filter_id)[0].checked = true
                                }

                                for (var s of params.Query.sources) {
                                    const filter_id = `#filter_src_${s}`
                                    jQuery(filter_id)[0].selected = true
                                }

                                if (params.Query.period.length == 2) {
                                    jQuery("#filter_period_begin")[0].valueAsDate =
                                        new Date(params.Query.period[0])
                                    jQuery("#filter_period_end")[0].valueAsDate =
                                        new Date(params.Query.period[1])
                                    jQuery("#filter_by_period_p")[0].checked = true
                                }

                                if (_.all(params.Query.terms, (x) => { return x.indexOf("(?i)") >= 0 })) {
                                    params.Query.terms =
                                        _.map(params.Query.terms, (x) => { return x.substring(4) })
                                    jQuery("#case_insensitive")[0].checked = true
                                } else {
                                    jQuery("#case_insensitive")[0].checked = false
                                }

                                jQuery("#search_terms")[0].value = params.Query.terms.join("\n")
                                jQuery("#search_id")[0].value = sid
                            } else {
                                const msg = `Error loading search results: ${res.Message}`
                                console.log(msg)
                                alert(msg)
                            }
                        },
                        'json'
                       ).fail((reply, status_text, xhr) => {
                           console.log(`Error searching: ${status_text} ${reply} ${xhr}`)
                       })
} // function search_load_results(sid, page)

function search_delete(id) {
    const addr = `/ajax/search/delete/${id}`

    const reply = $.get(addr,
                        {},
                        (res) => {
                            if (res.Status) {
                                const lid = `#search_${id}`
                                jQuery(lid)[0].remove()

                                const cur_id = Number.parseInt(jQuery("#search_id")[0].value)

                                if (id == cur_id) {
                                    clear_results()
                                    clear_filters()
                                    jQuery("#search_id")[0].value = ""
                                }
                            } else {
                                console.log(res.Message)
                                alert(res.Message)
                            }
                        },
                        'json')

    reply.fail((reply, status_text, xhr) => {
                           console.log(`Error searching: ${status_text} ${reply} ${xhr}`)
                       })
} // function search_delete(id)

function scale_images() {
    const selector = '#items img'
    const maxHeight = 300
    const maxWidth = 300

    $(selector).each(function () {
        const img = $(this)[0]
        if (img.width > maxWidth || img.height > maxHeight) {
            const size = shrink_img(img.width, img.height, maxWidth, maxHeight)

            img.width = size.width
            img.height = size.height
        }
    })
} // function scale_images()

// Found here: https://stackoverflow.com/questions/3971841/how-to-resize-images-proportionally-keeping-the-aspect-ratio#14731922
function shrink_img (srcWidth, srcHeight, maxWidth, maxHeight) {
    const ratio = Math.min(maxWidth / srcWidth, maxHeight / srcHeight)

    return { width: srcWidth * ratio, height: srcHeight * ratio }
} // function shrink_img(srcWidth, srcHeight, maxWidth, maxHeight)

function fix_links() {
    let links = $('#items a')

    for (var l of links) {
        l.target = '_blank'
    }
} // function fix_links()

function rate_item(item_id, rating) {
    const url = '/ajax/item_rate'

    const req = $.post(url,
                       { "item": item_id,
                         "rating": rating },
                       (res) => {
                           if (res.status) {
                               var icon = '';
                               switch (rating) {
                               case -1:
                                   icon = 'face-tired'
                                   break
                               case 1:
                                   icon = 'face-glasses'
                                   break
                               default:
                                   const msg = `Invalid rating: ${rating}`
                                   console.log(msg)
                                   alert(msg)
                                   return
                               }

                               const src = `/static/${icon}.png`
                               const cell = $(`#item_rating_${item_id}`)[0]

                               cell.innerHTML = `<img src="${src}" onclick="unrate_item(${item_id});" />`
                           } else {
                               alert(res.message)
                           }
                       },
                       'json')
} // function rate_item(item_id, rating)

function unrate_item(id) {
    const url = `/ajax/item_unrate/${id}`

    const req = $.get(url,
                      {},
                      (res) => {
                          if (!res.status) {
                              console.log(res.message)
                              alert(res.message)
                              return
                          }

                          $(`#item_rating_${id}`)[0].innerHTML = res.payload.cell
                      },
                      'json')
} // function unrate_item(id)

var item_cnt = 0

function load_items(cnt) {
    const url = `/ajax/items/${item_cnt}/${cnt}`

    const req = $.get(url,
                      {},
                      (res) => {
                          if (res.status) {
                              const tbody = $('#items')[0]
                              tbody.innerHTML += res.payload.content
                              item_cnt += cnt

                              if (res.payload.count == cnt && item_cnt < max_cnt) {
                                  console.log(`${item_cnt} Items already loaded, loading ${cnt} more items.`)
                                  window.setTimeout(load_items, 200, cnt)
                              }

                              window.setTimeout(fix_links, 10)
                              window.setTimeout(scale_images, 50)
                          } else {
                              console.log(res.message)
                              alert(res.message)
                          }
                      },
                      'json'
                     )

    req.fail(() => {
        alert("Error loading items")
    })
} // function load_items(cnt)

function add_tag(item_id) {
    const sel_id = `#item_tag_sel_${item_id}`
    const sel = $(sel_id)[0]
    const tag_id = sel.value
    const msg = `Add Tag ${tag_id} to Item ${item_id}`
    console.log(msg)
    alert(msg)
} // function add_tag(item_id)

// No sure if I really wand to go down that route...
function render_tag_single(item, tag) {
    return `<a href="/tags/${tag.id}">${tag.name}</a>
&nbsp;
<button onclick="remove_tag(${tag.id}, ${item.id});">
<img src="/static/delete.png" />
</button>`
} // function render_single_tag(item, tag)

function render_tags_for_item(item, tags) {
    const rendered_tags = []

    for (var t of tags) {
        const r = render_tag_single(item, t)
        rendered_tags.push(r)
    }

    return rendered_tags.join(" &nbsp; ")
} // function render_tags_for_item(item, tags)
