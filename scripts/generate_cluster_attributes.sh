#!/bin/bash

# Script to generate a domain update command with 3000 cluster attributes
# Each attribute is a unique city assigned to one of three clusters

set -e

DOMAIN="${1:-test25}"
NUM_ATTRIBUTES="${2:-3000}"

echo "Generating command with $NUM_ATTRIBUTES cluster attributes for domain: $DOMAIN"
echo ""

# Array of city names from around the world
# We'll use these as base names and add suffixes to reach 3000
CITIES=(
    # Major world cities
    "london" "paris" "berlin" "madrid" "rome" "moscow" "istanbul" "athens" "vienna" "prague"
    "amsterdam" "brussels" "copenhagen" "dublin" "helsinki" "lisbon" "oslo" "stockholm" "warsaw" "budapest"
    "tokyo" "beijing" "shanghai" "seoul" "bangkok" "singapore" "hong_kong" "mumbai" "delhi" "bangalore"
    "sydney" "melbourne" "auckland" "wellington" "brisbane" "perth" "adelaide" "canberra"
    "new_york" "los_angeles" "chicago" "houston" "phoenix" "philadelphia" "san_antonio" "san_diego" "dallas" "san_jose"
    "austin" "jacksonville" "fort_worth" "columbus" "charlotte" "san_francisco" "indianapolis" "seattle" "denver" "washington"
    "boston" "el_paso" "nashville" "detroit" "portland" "las_vegas" "memphis" "louisville" "baltimore" "milwaukee"
    "toronto" "montreal" "vancouver" "calgary" "ottawa" "edmonton" "winnipeg" "quebec_city" "hamilton" "victoria"
    "mexico_city" "guadalajara" "monterrey" "puebla" "tijuana" "leon" "ciudad_juarez" "zapopan" "merida" "san_luis_potosi"
    "sao_paulo" "rio_de_janeiro" "brasilia" "salvador" "fortaleza" "belo_horizonte" "manaus" "curitiba" "recife" "porto_alegre"
    "buenos_aires" "cordoba" "rosario" "mendoza" "la_plata" "mar_del_plata" "salta" "santa_fe" "san_juan" "resistencia"
    "bogota" "medellin" "cali" "barranquilla" "cartagena" "cucuta" "bucaramanga" "pereira" "ibague" "santa_marta"
    "lima" "arequipa" "trujillo" "chiclayo" "piura" "iquitos" "cusco" "huancayo" "tacna" "ica"
    "santiago" "valparaiso" "concepcion" "la_serena" "antofagasta" "temuco" "rancagua" "talca" "arica" "puerto_montt"
    "caracas" "maracaibo" "valencia" "barquisimeto" "maracay" "ciudad_guayana" "maturin" "barcelona" "puerto_la_cruz"
    "quito" "guayaquil" "cuenca" "santo_domingo" "machala" "duran" "portoviejo" "manta" "loja" "ambato"
    "cairo" "alexandria" "giza" "shubra_el_kheima" "port_said" "suez" "luxor" "al_mahallah_al_kubra" "tanta" "asyut"
    "johannesburg" "cape_town" "durban" "pretoria" "port_elizabeth" "pietermaritzburg" "benoni" "tembisa" "east_london" "vereeniging"
    "lagos" "kano" "ibadan" "kaduna" "port_harcourt" "benin_city" "maiduguri" "zaria" "aba" "jos"
    "nairobi" "mombasa" "kisumu" "nakuru" "eldoret" "thika" "malindi" "kitale" "garissa" "kakamega"
    "addis_ababa" "dire_dawa" "mekele" "gondar" "awasa" "bahir_dar" "dessie" "jimma" "jijiga" "shashamane"
    "kinshasa" "lubumbashi" "mbuji_mayi" "kisangani" "kananga" "likasi" "kolwezi" "tshikapa" "beni" "bukavu"
    "dar_es_salaam" "mwanza" "arusha" "dodoma" "mbeya" "morogoro" "tanga" "kahama" "tabora" "zanzibar"
    "kampala" "gulu" "lira" "mbarara" "jinja" "bwizibwera" "mbale" "mukono" "kasese" "masaka"
    "khartoum" "omdurman" "khartoum_north" "nyala" "port_sudan" "kassala" "el_obeid" "gedaref" "wad_medani" "al_qadarif"
    "accra" "kumasi" "tamale" "sekondi_takoradi" "ashaiman" "sunyani" "cape_coast" "obuasi" "teshie" "tema"
    "algiers" "oran" "constantine" "annaba" "blida" "batna" "djelfa" "setif" "sidi_bel_abbes" "biskra"
    "casablanca" "rabat" "fes" "marrakech" "agadir" "tangier" "meknes" "oujda" "kenitra" "tetouan"
    "tunis" "sfax" "sousse" "ettadhamen" "kairouan" "bizerte" "gabes" "ariana" "gafsa" "kasserine"
    "tripoli" "benghazi" "misrata" "tarhuna" "al_khums" "az_zawiya" "ajdabiya" "tobruk" "Sabha" "gharyan"
    "tehran" "mashhad" "isfahan" "karaj" "tabriz" "shiraz" "qom" "ahvaz" "kermanshah" "urmia"
    "baghdad" "basra" "mosul" "erbil" "kirkuk" "najaf" "karbala" "sulaymaniyah" "nasiriyah" "amarah"
    "riyadh" "jeddah" "mecca" "medina" "dammam" "taif" "tabuk" "khamis_mushait" "buraidah" "al_hofuf"
    "dubai" "abu_dhabi" "sharjah" "al_ain" "ajman" "ras_al_khaimah" "fujairah" "umm_al_quwain" "khor_fakkan" "dibba"
    "kabul" "kandahar" "herat" "mazar_i_sharif" "kunduz" "jalalabad" "lashkar_gah" "taloqan" "puli_khumri" "ghazni"
    "karachi" "lahore" "faisalabad" "rawalpindi" "multan" "hyderabad" "gujranwala" "peshawar" "quetta" "islamabad"
    "dhaka" "chittagong" "khulna" "rajshahi" "sylhet" "rangpur" "barisal" "comilla" "narayanganj" "gazipur"
    "kathmandu" "pokhara" "lalitpur" "bharatpur" "biratnagar" "birgunj" "dharan" "hetauda" "janakpur" "butwal"
    "yangon" "mandalay" "naypyidaw" "mawlamyine" "bago" "pathein" "monywa" "sittwe" "meiktila" "myeik"
    "hanoi" "ho_chi_minh_city" "haiphong" "da_nang" "bien_hoa" "hue" "nha_trang" "can_tho" "rach_gia" "qui_nhon"
    "manila" "quezon_city" "davao" "caloocan" "cebu_city" "zamboanga" "taguig" "antipolo" "pasig" "cagayan_de_oro"
    "jakarta" "surabaya" "bandung" "bekasi" "medan" "tangerang" "depok" "semarang" "palembang" "makassar"
    "kuala_lumpur" "george_town" "ipoh" "johor_bahru" "malacca_city" "shah_alam" "kota_kinabalu" "kuching" "petaling_jaya"
)

# Function to generate cluster attributes
generate_attributes() {
    local num=$1
    local attributes=()
    local city_index=0
    local suffix=0

    for i in $(seq 1 $num); do
        # Get base city name
        local base_city="${CITIES[$city_index]}"

        # Add suffix if we've used all base cities
        if [ $suffix -gt 0 ]; then
            local city_name="${base_city}_${suffix}"
        else
            local city_name="${base_city}"
        fi

        # Assign to a cluster (round-robin for even distribution)
        local cluster_num=$((i % 3))
        local cluster="cluster${cluster_num}"

        # Add attribute in format: location.city:cluster
        attributes+=("location.${city_name}:${cluster}")

        # Move to next city
        city_index=$(((city_index + 1) % ${#CITIES[@]}))

        # If we've cycled through all cities, increment suffix
        if [ $city_index -eq 0 ]; then
            suffix=$((suffix + 1))
        fi
    done

    # Join with commas
    local IFS=','
    echo "${attributes[*]}"
}

echo "Generating $NUM_ATTRIBUTES unique city-cluster mappings..."
ACTIVE_CLUSTERS=$(generate_attributes $NUM_ATTRIBUTES)

# Count the length to verify
ATTR_COUNT=$(echo "$ACTIVE_CLUSTERS" | grep -o "location\." | wc -l | tr -d ' ')
echo "Generated $ATTR_COUNT cluster attributes"
echo ""

# Generate the full command
COMMAND="./cadence --transport grpc --ad localhost:7833 --domain $DOMAIN domain update --active_clusters '$ACTIVE_CLUSTERS'"

# Save to file since it's too long to display
OUTPUT_FILE="update_${DOMAIN}_${NUM_ATTRIBUTES}_attrs.sh"
echo "#!/bin/bash" > "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"
echo "# Auto-generated domain update with $NUM_ATTRIBUTES cluster attributes" >> "$OUTPUT_FILE"
echo "# Domain: $DOMAIN" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"
echo "$COMMAND" >> "$OUTPUT_FILE"

chmod +x "$OUTPUT_FILE"

echo "========================================="
echo "Command saved to: $OUTPUT_FILE"
echo "========================================="
echo ""
echo "Command length: ${#COMMAND} characters"
echo "Attribute string length: ${#ACTIVE_CLUSTERS} characters"
echo ""
echo "To execute:"
echo "  ./$OUTPUT_FILE"
echo ""
echo "Preview (first 500 chars):"
echo "${COMMAND:0:500}..."
echo ""
echo "Distribution:"
echo "  - cluster0: ~$((NUM_ATTRIBUTES / 3)) attributes"
echo "  - cluster1: ~$((NUM_ATTRIBUTES / 3)) attributes"
echo "  - cluster2: ~$((NUM_ATTRIBUTES / 3)) attributes"
