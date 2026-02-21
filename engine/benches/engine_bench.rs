use criterion::{black_box, criterion_group, criterion_main, Criterion};
use std::time::Duration;

use realpolitik::board::province::Power;
use realpolitik::eval::{evaluate, evaluate_all};
use realpolitik::movegen::movement::legal_orders;
use realpolitik::protocol::dfen::parse_dfen;
use realpolitik::resolve::Resolver;
use std::sync::atomic::AtomicBool;

use realpolitik::search::cartesian::search;
use realpolitik::search::regret_matching_search;

const INITIAL_DFEN: &str = "1901sm/Aavie,Aabud,Aftri,Eflon,Efedi,Ealvp,Ffbre,Fapar,Famar,Gfkie,Gaber,Gamun,Ifnap,Iarom,Iaven,Rfstp.sc,Ramos,Rawar,Rfsev,Tfank,Tacon,Tasmy/Abud,Atri,Avie,Eedi,Elon,Elvp,Fbre,Fmar,Fpar,Gber,Gkie,Gmun,Inap,Irom,Iven,Rmos,Rsev,Rstp,Rwar,Tank,Tcon,Tsmy,Nbel,Nbul,Nden,Ngre,Nhol,Nnwy,Npor,Nrum,Nser,Nspa,Nswe,Ntun/-";

fn bench_evaluate(c: &mut Criterion) {
    let state = parse_dfen(INITIAL_DFEN).unwrap();
    c.bench_function("evaluate_single_power", |b| {
        b.iter(|| evaluate(black_box(Power::Austria), black_box(&state)))
    });
}

fn bench_evaluate_all(c: &mut Criterion) {
    let state = parse_dfen(INITIAL_DFEN).unwrap();
    c.bench_function("evaluate_all_7_powers", |b| {
        b.iter(|| evaluate_all(black_box(&state)))
    });
}

fn bench_resolve_initial(c: &mut Criterion) {
    let state = parse_dfen(INITIAL_DFEN).unwrap();
    // Build a realistic order set: all 22 units hold
    use realpolitik::board::order::{Location, Order, OrderUnit};
    use realpolitik::board::province::{ALL_PROVINCES, PROVINCE_COUNT};
    use realpolitik::board::unit::UnitType;

    let mut orders = Vec::new();
    for i in 0..PROVINCE_COUNT {
        if let Some((power, unit_type)) = state.units[i] {
            let prov = ALL_PROVINCES[i];
            let coast = state.fleet_coast[i].unwrap_or(realpolitik::board::province::Coast::None);
            orders.push((
                Order::Hold {
                    unit: OrderUnit {
                        unit_type,
                        location: Location::with_coast(prov, coast),
                    },
                },
                power,
            ));
        }
    }

    c.bench_function("resolve_22_holds", |b| {
        let mut resolver = Resolver::new(32);
        b.iter(|| resolver.resolve(black_box(&orders), black_box(&state)))
    });
}

fn bench_resolve_with_moves(c: &mut Criterion) {
    let state = parse_dfen(INITIAL_DFEN).unwrap();
    use realpolitik::board::order::{Location, Order, OrderUnit};
    use realpolitik::board::province::{Coast, Province};
    use realpolitik::board::unit::UnitType;

    fn army(prov: Province) -> OrderUnit {
        OrderUnit {
            unit_type: UnitType::Army,
            location: Location::new(prov),
        }
    }
    fn fleet(prov: Province) -> OrderUnit {
        OrderUnit {
            unit_type: UnitType::Fleet,
            location: Location::new(prov),
        }
    }
    fn fleet_coast(prov: Province, coast: Coast) -> OrderUnit {
        OrderUnit {
            unit_type: UnitType::Fleet,
            location: Location::with_coast(prov, coast),
        }
    }

    // A realistic Spring 1901 order set with moves, supports, and holds
    let orders = vec![
        // Austria
        (
            Order::Move {
                unit: army(Province::Vie),
                dest: Location::new(Province::Gal),
            },
            Power::Austria,
        ),
        (
            Order::Move {
                unit: army(Province::Bud),
                dest: Location::new(Province::Ser),
            },
            Power::Austria,
        ),
        (
            Order::Move {
                unit: fleet(Province::Tri),
                dest: Location::new(Province::Alb),
            },
            Power::Austria,
        ),
        // England
        (
            Order::Move {
                unit: fleet(Province::Lon),
                dest: Location::new(Province::Nth),
            },
            Power::England,
        ),
        (
            Order::Move {
                unit: fleet(Province::Edi),
                dest: Location::new(Province::Nrg),
            },
            Power::England,
        ),
        (
            Order::Move {
                unit: army(Province::Lvp),
                dest: Location::new(Province::Yor),
            },
            Power::England,
        ),
        // France
        (
            Order::Move {
                unit: fleet(Province::Bre),
                dest: Location::new(Province::Mao),
            },
            Power::France,
        ),
        (
            Order::Move {
                unit: army(Province::Par),
                dest: Location::new(Province::Bur),
            },
            Power::France,
        ),
        (
            Order::Move {
                unit: army(Province::Mar),
                dest: Location::new(Province::Pie),
            },
            Power::France,
        ),
        // Germany
        (
            Order::Move {
                unit: fleet(Province::Kie),
                dest: Location::new(Province::Den),
            },
            Power::Germany,
        ),
        (
            Order::Move {
                unit: army(Province::Ber),
                dest: Location::new(Province::Kie),
            },
            Power::Germany,
        ),
        (
            Order::Move {
                unit: army(Province::Mun),
                dest: Location::new(Province::Ruh),
            },
            Power::Germany,
        ),
        // Italy
        (
            Order::Move {
                unit: fleet(Province::Nap),
                dest: Location::new(Province::Ion),
            },
            Power::Italy,
        ),
        (
            Order::Move {
                unit: army(Province::Rom),
                dest: Location::new(Province::Apu),
            },
            Power::Italy,
        ),
        (
            Order::Move {
                unit: army(Province::Ven),
                dest: Location::new(Province::Tri),
            },
            Power::Italy,
        ),
        // Russia
        (
            Order::Move {
                unit: fleet_coast(Province::Stp, Coast::South),
                dest: Location::new(Province::Bot),
            },
            Power::Russia,
        ),
        (
            Order::Move {
                unit: army(Province::Mos),
                dest: Location::new(Province::Ukr),
            },
            Power::Russia,
        ),
        (
            Order::Move {
                unit: army(Province::War),
                dest: Location::new(Province::Gal),
            },
            Power::Russia,
        ),
        (
            Order::Move {
                unit: fleet(Province::Sev),
                dest: Location::new(Province::Bla),
            },
            Power::Russia,
        ),
        // Turkey
        (
            Order::Move {
                unit: fleet(Province::Ank),
                dest: Location::new(Province::Bla),
            },
            Power::Turkey,
        ),
        (
            Order::Move {
                unit: army(Province::Con),
                dest: Location::new(Province::Bul),
            },
            Power::Turkey,
        ),
        (
            Order::Move {
                unit: army(Province::Smy),
                dest: Location::new(Province::Con),
            },
            Power::Turkey,
        ),
    ];

    c.bench_function("resolve_22_spring_moves", |b| {
        let mut resolver = Resolver::new(32);
        b.iter(|| resolver.resolve(black_box(&orders), black_box(&state)))
    });
}

fn bench_search_austria_200ms(c: &mut Criterion) {
    let state = parse_dfen(INITIAL_DFEN).unwrap();
    let mut group = c.benchmark_group("search");
    group.sample_size(10);
    group.measurement_time(Duration::from_secs(10));
    group.bench_function("austria_200ms", |b| {
        b.iter(|| {
            let mut out = Vec::new();
            search(
                black_box(Power::Austria),
                black_box(&state),
                Duration::from_millis(200),
                &mut out,
                &AtomicBool::new(false),
            )
        })
    });
    group.finish();
}

fn bench_movegen_austria(c: &mut Criterion) {
    let state = parse_dfen(INITIAL_DFEN).unwrap();
    use realpolitik::board::province::{ALL_PROVINCES, PROVINCE_COUNT};

    c.bench_function("movegen_austria_3_units", |b| {
        b.iter(|| {
            for i in 0..PROVINCE_COUNT {
                if let Some((p, _)) = state.units[i] {
                    if p == Power::Austria {
                        let _ = legal_orders(black_box(ALL_PROVINCES[i]), black_box(&state));
                    }
                }
            }
        })
    });
}

fn bench_movegen_all_powers(c: &mut Criterion) {
    let state = parse_dfen(INITIAL_DFEN).unwrap();
    use realpolitik::board::province::{ALL_PROVINCES, PROVINCE_COUNT};

    c.bench_function("movegen_all_22_units", |b| {
        b.iter(|| {
            for i in 0..PROVINCE_COUNT {
                if state.units[i].is_some() {
                    let _ = legal_orders(black_box(ALL_PROVINCES[i]), black_box(&state));
                }
            }
        })
    });
}

fn bench_rm_search_austria_500ms(c: &mut Criterion) {
    let state = parse_dfen(INITIAL_DFEN).unwrap();
    let mut group = c.benchmark_group("rm_search");
    group.sample_size(10);
    group.measurement_time(Duration::from_secs(15));
    group.bench_function("austria_500ms", |b| {
        b.iter(|| {
            let mut out = Vec::new();
            regret_matching_search(
                black_box(Power::Austria),
                black_box(&state),
                Duration::from_millis(500),
                &mut out,
                None,
                100,
                None,
                &AtomicBool::new(false),
            )
        })
    });
    group.finish();
}

fn bench_rm_search_russia_500ms(c: &mut Criterion) {
    let state = parse_dfen(INITIAL_DFEN).unwrap();
    let mut group = c.benchmark_group("rm_search");
    group.sample_size(10);
    group.measurement_time(Duration::from_secs(15));
    group.bench_function("russia_500ms", |b| {
        b.iter(|| {
            let mut out = Vec::new();
            regret_matching_search(
                black_box(Power::Russia),
                black_box(&state),
                Duration::from_millis(500),
                &mut out,
                None,
                100,
                None,
                &AtomicBool::new(false),
            )
        })
    });
    group.finish();
}

fn bench_resolve_then_evaluate(c: &mut Criterion) {
    let state = parse_dfen(INITIAL_DFEN).unwrap();
    use realpolitik::board::order::{Location, Order, OrderUnit};
    use realpolitik::board::province::{Coast, Province};
    use realpolitik::board::unit::UnitType;
    use realpolitik::resolve::apply_resolution;

    fn army(prov: Province) -> OrderUnit {
        OrderUnit {
            unit_type: UnitType::Army,
            location: Location::new(prov),
        }
    }
    fn fleet(prov: Province) -> OrderUnit {
        OrderUnit {
            unit_type: UnitType::Fleet,
            location: Location::new(prov),
        }
    }
    fn fleet_coast(prov: Province, coast: Coast) -> OrderUnit {
        OrderUnit {
            unit_type: UnitType::Fleet,
            location: Location::with_coast(prov, coast),
        }
    }

    let orders = vec![
        (
            Order::Move {
                unit: army(Province::Vie),
                dest: Location::new(Province::Gal),
            },
            Power::Austria,
        ),
        (
            Order::Move {
                unit: army(Province::Bud),
                dest: Location::new(Province::Ser),
            },
            Power::Austria,
        ),
        (
            Order::Move {
                unit: fleet(Province::Tri),
                dest: Location::new(Province::Alb),
            },
            Power::Austria,
        ),
        (
            Order::Move {
                unit: fleet(Province::Lon),
                dest: Location::new(Province::Nth),
            },
            Power::England,
        ),
        (
            Order::Move {
                unit: fleet(Province::Edi),
                dest: Location::new(Province::Nrg),
            },
            Power::England,
        ),
        (
            Order::Move {
                unit: army(Province::Lvp),
                dest: Location::new(Province::Yor),
            },
            Power::England,
        ),
        (
            Order::Move {
                unit: fleet(Province::Bre),
                dest: Location::new(Province::Mao),
            },
            Power::France,
        ),
        (
            Order::Move {
                unit: army(Province::Par),
                dest: Location::new(Province::Bur),
            },
            Power::France,
        ),
        (
            Order::Move {
                unit: army(Province::Mar),
                dest: Location::new(Province::Pie),
            },
            Power::France,
        ),
        (
            Order::Move {
                unit: fleet(Province::Kie),
                dest: Location::new(Province::Den),
            },
            Power::Germany,
        ),
        (
            Order::Move {
                unit: army(Province::Ber),
                dest: Location::new(Province::Kie),
            },
            Power::Germany,
        ),
        (
            Order::Move {
                unit: army(Province::Mun),
                dest: Location::new(Province::Ruh),
            },
            Power::Germany,
        ),
        (
            Order::Move {
                unit: fleet(Province::Nap),
                dest: Location::new(Province::Ion),
            },
            Power::Italy,
        ),
        (
            Order::Move {
                unit: army(Province::Rom),
                dest: Location::new(Province::Apu),
            },
            Power::Italy,
        ),
        (
            Order::Move {
                unit: army(Province::Ven),
                dest: Location::new(Province::Tri),
            },
            Power::Italy,
        ),
        (
            Order::Move {
                unit: fleet_coast(Province::Stp, Coast::South),
                dest: Location::new(Province::Bot),
            },
            Power::Russia,
        ),
        (
            Order::Move {
                unit: army(Province::Mos),
                dest: Location::new(Province::Ukr),
            },
            Power::Russia,
        ),
        (
            Order::Move {
                unit: army(Province::War),
                dest: Location::new(Province::Gal),
            },
            Power::Russia,
        ),
        (
            Order::Move {
                unit: fleet(Province::Sev),
                dest: Location::new(Province::Bla),
            },
            Power::Russia,
        ),
        (
            Order::Move {
                unit: fleet(Province::Ank),
                dest: Location::new(Province::Bla),
            },
            Power::Turkey,
        ),
        (
            Order::Move {
                unit: army(Province::Con),
                dest: Location::new(Province::Bul),
            },
            Power::Turkey,
        ),
        (
            Order::Move {
                unit: army(Province::Smy),
                dest: Location::new(Province::Con),
            },
            Power::Turkey,
        ),
    ];

    c.bench_function("resolve_then_evaluate_cycle", |b| {
        let mut resolver = Resolver::new(32);
        let mut scratch = state.clone();
        b.iter(|| {
            let (results, dislodged) = resolver.resolve(black_box(&orders), black_box(&state));
            scratch.clone_from(&state);
            apply_resolution(&mut scratch, &results, &dislodged);
            evaluate(Power::Austria, &scratch)
        })
    });
}

fn bench_board_state_clone(c: &mut Criterion) {
    let state = parse_dfen(INITIAL_DFEN).unwrap();
    c.bench_function("board_state_clone", |b| {
        b.iter(|| black_box(&state).clone())
    });
}

criterion_group!(
    benches,
    bench_evaluate,
    bench_evaluate_all,
    bench_resolve_initial,
    bench_resolve_with_moves,
    bench_search_austria_200ms,
    bench_movegen_austria,
    bench_movegen_all_powers,
    bench_rm_search_austria_500ms,
    bench_rm_search_russia_500ms,
    bench_resolve_then_evaluate,
    bench_board_state_clone,
);
criterion_main!(benches);
