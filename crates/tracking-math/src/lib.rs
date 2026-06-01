fn max_f32(a: f32, b: f32) -> f32 {
    if a > b {
        a
    } else {
        b
    }
}

fn min_f32(a: f32, b: f32) -> f32 {
    if a < b {
        a
    } else {
        b
    }
}

#[no_mangle]
pub extern "C" fn iou_xywh(ax: f32, ay: f32, aw: f32, ah: f32, bx: f32, by: f32, bw: f32, bh: f32) -> f32 {
    if aw <= 0.0 || ah <= 0.0 || bw <= 0.0 || bh <= 0.0 {
        return 0.0;
    }
    let ax2 = ax + aw;
    let ay2 = ay + ah;
    let bx2 = bx + bw;
    let by2 = by + bh;
    let ix1 = max_f32(ax, bx);
    let iy1 = max_f32(ay, by);
    let ix2 = min_f32(ax2, bx2);
    let iy2 = min_f32(ay2, by2);
    let iw = max_f32(0.0, ix2 - ix1);
    let ih = max_f32(0.0, iy2 - iy1);
    let inter = iw * ih;
    if inter <= 0.0 {
        return 0.0;
    }
    let union = aw * ah + bw * bh - inter;
    if union <= 0.0 {
        0.0
    } else {
        inter / union
    }
}

#[no_mangle]
pub extern "C" fn segment_intersection_start(track_start: i32, track_end: i32, segment_start: i32, segment_end: i32) -> i32 {
    if track_end < segment_start || segment_end < track_start {
        return 0;
    }
    if track_start > segment_start {
        track_start
    } else {
        segment_start
    }
}

#[no_mangle]
pub extern "C" fn segment_intersection_end(track_start: i32, track_end: i32, segment_start: i32, segment_end: i32) -> i32 {
    if track_end < segment_start || segment_end < track_start {
        return 0;
    }
    if track_end < segment_end {
        track_end
    } else {
        segment_end
    }
}

#[no_mangle]
pub extern "C" fn segment_intersects(track_start: i32, track_end: i32, segment_start: i32, segment_end: i32) -> i32 {
    if track_end < segment_start || segment_end < track_start {
        0
    } else {
        1
    }
}
