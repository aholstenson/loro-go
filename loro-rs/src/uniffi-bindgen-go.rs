fn main() {
    #[cfg(feature = "cli")]
    let _ = uniffi_bindgen_go::main();
}
