// Time-stamp: <2024-11-02 21:49:42 krylon>
// -*- mode: javascript; coding: utf-8; -*-
// Copyright 2015-2020 Benjamin Walkenhorst <krylon@gmx.net>
//
// This file has grown quite a bit larger than I had anticipated.
// It is not a /big/ problem right now, but in the long run, I will have to
// break this thing up into several smaller files.

'use strict';

function defined(x) {
    return undefined !== x && null !== x
}

function fmtDateNumber(n) {
    return (n < 10 ? '0' : '') + n.toString()
} // function fmtDateNumber(n)

function timeStampString(t) {
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

function fmtDuration(seconds) {
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

function beaconLoop() {
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

function beaconToggle() {
    settings.beacon.active = !settings.beacon.active
    saveSetting('beacon', 'active', settings.beacon.active)

    if (!settings.beacon.active) {
        const beaconDiv = $('#beacon')[0]
        beaconDiv.innerHTML = 'Beacon is suspended'
        beaconDiv.classList.remove('error')
    }
} // function beaconToggle()

function toggle_hide_boring() {
    const state = !settings.news.hideBoring
    settings.news.hideBoring = state
    saveSetting('news', 'hideBoring', state)
    $("#toggle_hide_boring")[0].checked = state
} // function toggle_hide_boring()

/*
  The ‘content’ attribute of Window objects is deprecated.  Please use ‘window.top’ instead. interact.js:125:8
  Ignoring get or set of property that has [LenientThis] because the “this” object is incorrect. interact.js:125:8

*/

function db_maintenance() {
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
function shrink_img(srcWidth, srcHeight, maxWidth, maxHeight) {
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
                               msg_add(res.message)
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
                              msg_add(res.message, 2)
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
                      {
                          "hideBoring": settings.news.hideBoring,
                      },
                      (res) => {
                          if (res.status) {
                              const tbody = $('#items')[0]
                              tbody.innerHTML += res.payload.content
                              item_cnt += cnt

                              if (res.payload.count > 0 && item_cnt < max_cnt) {
                                  const msg = `${item_cnt} Items already loaded, loading ${cnt} more items.`
                                  console.log(msg)
                                  msg_add(msg, 2)
                                  window.setTimeout(load_items, 200, cnt)
                              }

                              window.setTimeout(fix_links, 10)
                              window.setTimeout(scale_images, 50)
                          } else {
                              console.log(res.message)
                              msg_add(res.message, 2)
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
    const opt_id = `#tag_menu_item_${item_id}_opt_${tag_id}`
    const url = `/ajax/tag/link/${tag_id}/${item_id}`
    const msg = `Add Tag ${tag_id} to Item ${item_id}`
    console.log(msg)

    const req = $.get(
        url,
        {},
        (res) => {
            if (res.status) {
                const item = JSON.parse(res.payload.item)
                const tag = JSON.parse(res.payload.tag)

                const div_id = `#item_tags_${item_id}`
                const div = $(div_id)[0]

                const snippet = render_tag_single(item, tag)

                div.innerHTML += snippet

                const opt = $(opt_id)[0]
                opt.disabled = true
            } else {
                console.log(res.message)
                msg_add(res.message)
            }
        },
        'json'
    )
} // function add_tag(item_id)

// No sure if I really wand to go down that route...
function render_tag_single(item, tag) {
    return `<span id="tag_link_${item.id}_${tag.id}">
<a href="/tags/${tag.id}">${tag.name}</a>
&nbsp;
<img src="/static/delete.png" onclick="remove_tag(${tag.id}, ${item.id});" />
</span> `
} // function render_single_tag(item, tag)

function render_tags_for_item(item, tags) {
    const rendered_tags = []

    for (var t of tags) {
        const r = render_tag_single(item, t)
        rendered_tags.push(r)
    }

    return rendered_tags.join(" &nbsp; ")
} // function render_tags_for_item(item, tags)

function remove_tag(tag_id, item_id) {
    const url = `/ajax/tag/unlink/${tag_id}/${item_id}`
    $.get(
        url,
        {},
        (res) => {
            if (res.status) {
                const span_id = `#tag_link_${item_id}_${tag_id}`
                $(span_id).remove()
            } else {
                console.log(res.message)
                msg_add(res.message, 2)
            }
        },
        'json')
} // function remove_tag(tag_id, item_id)

function attach_tag_to_item(tag_id, item_id, span_id) {
    const url = `/ajax/tag/link/${tag_id}/${item_id}`
    const msg = `Add Tag ${tag_id} to Item ${item_id}`

    const req = $.get(
        url,
        {},
        (res) => {
            if (res.status) {
                const item = JSON.parse(res.payload.item)
                const tag = JSON.parse(res.payload.tag)
                const div_id = `#item_tags_${item_id}`
                const div = $(div_id)[0]
                const snippet = render_tag_single(item, tag)
                div.innerHTML += snippet
                const sugg = $(`#${span_id}`)[0]
                sugg.remove()
            } else {
                console.log(res.message)
                msg_add(res.message, 2)
            }
        },
        'json'
    )
} // function attach_tag_to_item(tag_id, item_id)

const max_msg_cnt = 5

function msg_clear() {
    $('#msg_tbl')[0].innerHTML = ''
} // function msg_clear()

function msg_add(msg, level=1) {
    const row = `<tr><td>${new Date()}</td><td>${level}</td><td>${msg}</td><td></td></tr>`
    const msg_tbl = $('#msg_tbl')[0]

    const rows = $('#msg_tbl tr')
    let i = 0
    let cnt = rows.length
    while (cnt >= max_msg_cnt) {
        rows[i].remove()
        i++
        cnt--
    }

    msg_tbl.innerHTML += row
} // function msg_add(msg)

function toggle_feed_active(feed_id) {
    const switch_id = `#check_feed_active_${feed_id}`
    const ckbox = $(switch_id)[0]
    const flag = ckbox.checked
    // msg_add(`Feed ${feed_id} is now ${flag ? "Active" : "Suspended"}`, 3)
    const url = `/ajax/feed/${feed_id}/toggle`
    const req = $.get(
        url,
        {},
        (res) => {
            if (res.status) {
                msg_add(`Active flag was toggled successfully`, 1)
            } else {
                msg_add(res.message, 3)
            }
        },
        'json')

    req.fail((reply, status, xhr) => {
        msg_add(status, 3)
    })
} // function toggle_feed_active(feed_id)

function blacklist_add_pattern() {
    const url = "/ajax/blacklist/add"
    const txtid = "#blacklist-pattern"
    const node = $(txtid)[0]
    const pat = node.value
    try {
        const re = RegExp(pat)
        $.post(
            url,
            { "pattern": pat },
            (res) => {
                if (res.status) {
                    const tr = `<tr id="bl_pat_${res.payload.pattern.id}">
  <td>
    ${pat}
  </td>
  <td>0</td>
  <td>
    <button type="button" class="btn">Edit</button>
    <button type="button" class="btn btn-danger">Remove</button>
  </td>
</tr>`
                    const tbl = $("#blacklist-table")[0]
                    tbl.innerHTML += tr
                } else {
                    msg_add(res.message, 3)
                }
            },
            'json')
    } catch (e) {
        if (e instanceof SyntaxError) {
            // ...
            const msg = `Invalid pattern: ${e}`
            msg_add(msg, 3)
        } else {
            throw e
        }
    }
} // function blacklist_add_pattern()
