//! Read-only driver preflight. Report advertised decoder configurations and
//! the window's actual display mode; only playback can prove smooth rendering.

#[derive(Default, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct Capabilities {
    source: &'static str,
    adapter: Option<String>,
    display_width: Option<u32>,
    display_height: Option<u32>,
    hdr_enabled: Option<bool>,
    hardware_decode: Option<bool>,
    decoder_profiles: Vec<String>,
    reason: Option<&'static str>,
}

pub fn probe(hwnd: isize, codec: &str, width: u32, height: u32) -> Capabilities {
    #[cfg(windows)]
    {
        windows_probe(hwnd, codec, width, height).unwrap_or_else(|_| Capabilities {
            source: "windows-dxgi-d3d11",
            reason: Some("driver_unavailable"),
            ..Default::default()
        })
    }
    #[cfg(not(windows))]
    {
        let _ = (hwnd, codec, width, height);
        Capabilities {
            source: "unavailable",
            reason: Some("platform_unavailable"),
            ..Default::default()
        }
    }
}

#[cfg(windows)]
fn windows_probe(
    hwnd: isize,
    codec: &str,
    width: u32,
    height: u32,
) -> windows::core::Result<Capabilities> {
    use windows::{
        core::{Interface, PCWSTR},
        Win32::{
            Foundation::{HMODULE, HWND},
            Graphics::{
                Direct3D::D3D_DRIVER_TYPE_UNKNOWN,
                Direct3D11::*,
                Dxgi::{Common::*, *},
                Gdi::{
                    EnumDisplaySettingsW, MonitorFromWindow, DEVMODEW, ENUM_CURRENT_SETTINGS,
                    MONITOR_DEFAULTTOPRIMARY,
                },
            },
        },
    };
    // This is display enumeration, never an attempt to move or control a window.
    let monitor = unsafe { MonitorFromWindow(HWND(hwnd as *mut _), MONITOR_DEFAULTTOPRIMARY) };
    let factory: IDXGIFactory1 = unsafe { CreateDXGIFactory1()? };
    for adapter_index in 0..32 {
        let Ok(adapter) = (unsafe { factory.EnumAdapters1(adapter_index) }) else {
            break;
        };
        for output_index in 0..32 {
            let Ok(output) = (unsafe { adapter.EnumOutputs(output_index) }) else {
                break;
            };
            let desc = unsafe { output.GetDesc()? };
            if desc.Monitor != monitor || !desc.AttachedToDesktop.as_bool() {
                continue;
            }
            let adapter_desc = unsafe { adapter.GetDesc1()? };
            // Desktop coordinates can be DPI-virtualised in the headless CLI.
            // EnumDisplaySettings always returns physical pixels for this output.
            let mut mode = DEVMODEW {
                dmSize: std::mem::size_of::<DEVMODEW>() as u16,
                ..Default::default()
            };
            let measured_mode = unsafe {
                EnumDisplaySettingsW(
                    PCWSTR(desc.DeviceName.as_ptr()),
                    ENUM_CURRENT_SETTINGS,
                    &mut mode,
                )
            }
            .as_bool();
            let mut result = Capabilities {
                source: "windows-dxgi-d3d11",
                adapter: Some(
                    String::from_utf16_lossy(&adapter_desc.Description)
                        .trim_end_matches('\0')
                        .to_string(),
                ),
                display_width: measured_mode.then_some(mode.dmPelsWidth),
                display_height: measured_mode.then_some(mode.dmPelsHeight),
                ..Default::default()
            };
            // DXGI reports the CURRENT output colour space, not a guessed HDR
            // capability derived from 10-bit pixels or the media's filename.
            if let Ok(output) = output.cast::<IDXGIOutput6>() {
                if let Ok(desc) = unsafe { output.GetDesc1() } {
                    result.hdr_enabled = match desc.ColorSpace {
                        DXGI_COLOR_SPACE_RGB_FULL_G2084_NONE_P2020 => Some(true),
                        DXGI_COLOR_SPACE_RGB_FULL_G22_NONE_P709 => Some(false),
                        _ => None,
                    };
                }
            }
            if width == 0 || height == 0 || width > 16384 || height > 16384 {
                result.reason = Some("media_unmeasured");
                return Ok(result);
            }
            let profiles = match codec.to_ascii_lowercase().as_str() {
                "h264" | "avc" => vec![(
                    D3D11_DECODER_PROFILE_H264_VLD_NOFGT,
                    DXGI_FORMAT_NV12,
                    "H.264 8-bit",
                )],
                "hevc" | "h265" => vec![
                    (
                        D3D11_DECODER_PROFILE_HEVC_VLD_MAIN,
                        DXGI_FORMAT_NV12,
                        "HEVC Main 8-bit",
                    ),
                    (
                        D3D11_DECODER_PROFILE_HEVC_VLD_MAIN10,
                        DXGI_FORMAT_P010,
                        "HEVC Main 10-bit",
                    ),
                ],
                "vp9" => vec![
                    (
                        D3D11_DECODER_PROFILE_VP9_VLD_PROFILE0,
                        DXGI_FORMAT_NV12,
                        "VP9 8-bit",
                    ),
                    (
                        D3D11_DECODER_PROFILE_VP9_VLD_10BIT_PROFILE2,
                        DXGI_FORMAT_P010,
                        "VP9 10-bit",
                    ),
                ],
                "av1" => vec![
                    (
                        D3D11_DECODER_PROFILE_AV1_VLD_PROFILE0,
                        DXGI_FORMAT_NV12,
                        "AV1 8-bit",
                    ),
                    (
                        D3D11_DECODER_PROFILE_AV1_VLD_PROFILE0,
                        DXGI_FORMAT_P010,
                        "AV1 10-bit",
                    ),
                ],
                _ => {
                    result.reason = Some("codec_unprobed");
                    return Ok(result);
                }
            };
            let adapter: IDXGIAdapter = adapter.cast()?;
            let mut device = None;
            unsafe {
                D3D11CreateDevice(
                    &adapter,
                    D3D_DRIVER_TYPE_UNKNOWN,
                    HMODULE::default(),
                    D3D11_CREATE_DEVICE_VIDEO_SUPPORT,
                    None,
                    D3D11_SDK_VERSION,
                    Some(&mut device),
                    None,
                    None,
                )?;
            }
            let video: ID3D11VideoDevice = device
                .ok_or(windows::core::Error::from_hresult(windows::core::HRESULT(
                    0x80004005u32 as i32,
                )))?
                .cast()?;
            let mut read_all = true;
            for (profile, format, label) in profiles {
                let desc = D3D11_VIDEO_DECODER_DESC {
                    Guid: profile,
                    SampleWidth: width,
                    SampleHeight: height,
                    OutputFormat: format,
                };
                match unsafe { video.GetVideoDecoderConfigCount(&desc) } {
                    Ok(count) if count > 0 => result.decoder_profiles.push(label.to_string()),
                    Ok(_) => (),
                    Err(_) => read_all = false,
                }
            }
            result.hardware_decode = if !result.decoder_profiles.is_empty() {
                Some(true)
            } else if read_all {
                Some(false)
            } else {
                None
            };
            return Ok(result);
        }
    }
    Err(windows::core::Error::from_hresult(windows::core::HRESULT(
        0x80004005u32 as i32,
    )))
}
