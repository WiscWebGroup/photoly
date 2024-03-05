<script setup>
import { RouterLink, RouterView } from 'vue-router'
import router from "../router/index.js";
import axios from "axios";
import { ref, defineComponent } from 'vue';
import '@/resources/RepeatMarker.js';
</script>
<template>
  <div style="position: relative;">
    <n-spin :show="!showPhotoIsLoaded" style="">
        <n-skeleton v-show="!showPhotoIsLoaded" :sharp="false" style="width:99vw; height:92vh; opacity: 0.8;" />
    </n-spin>
    <n-modal v-model:show="loadImage">
        <div style="min-width: 40vw ;max-width: 85vw; padding: 0; border-radius: 0.375rem;">
            <n-card
                style="padding: 0; border-radius: 0.375rem;"
                    :bordered="false"
                    size="huge"
                    role="dialog"
                    aria-modal="true"
                >
            <img id="showImg" v-show="loadImage" v-bind:src="baseUPhoto + userToken + '?photoId=' + loadImageId" 
                @click.left.native="() => {loadImage = false}" style="object-fit: fill; border-radius: 0.375rem; height:85vh;"/>
            </n-card>
        </div>
    </n-modal>
    
    
    <div v-show="showPhotoIsLoaded" id="img_map" style="height: 92vh; width: 99vw;"></div>
  </div>
</template>

<script>


export default defineComponent({
  data() {
    return {
        showPhotoIsLoaded: false,
        loadImage: false,
        loadImageId: -1,
        default_GPS_Latitude: "19.0000",
        default_GPS_Longitude: "10.0000",
        list_photos_to_show: [],
        baseUThumbnail: import.meta.env.VITE_APP_BASE_URL + "/photo/renderThumbnail/" ,
        baseUPhoto: import.meta.env.VITE_APP_BASE_URL + "/photo/render/" ,
        baseQR: import.meta.env.VITE_APP_BASE_URL + "/photo/getQR2?",
        userToken: localStorage.getItem("HRD-Token"),

    }
  },
  setup () {
    return {
      
    }
  },
  methods: {
    openShowPhotoModal(id) {
        this.loadImageId = id;
        this.loadImage = true;
    },
    openMapInNewTab(lat, lon) {
        var url = "https://maps.google.com/?q=" + lat + "," + lon;
        window.open(url, '_blank').focus();
    },
    setUpMap() {
        this.globalMap = null;
        this.globalMap = L.map('img_map', {attributionControl: false,}).setView([this.default_GPS_Latitude, this.default_GPS_Longitude], 3);
        var myRepeatingMarkers = L.gridLayer.repeatedMarkers().addTo(this.globalMap);
        var customDefault = L.icon({
            iconUrl: 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAABkAAAApCAYAAADAk4LOAAAFgUlEQVR4Aa1XA5BjWRTN2oW17d3YaZtr2962HUzbDNpjszW24mRt28p47v7zq/bXZtrp/lWnXr337j3nPCe85NcypgSFdugCpW5YoDAMRaIMqRi6aKq5E3YqDQO3qAwjVWrD8Ncq/RBpykd8oZUb/kaJutow8r1aP9II0WmLKLIsJyv1w/kqw9Ch2MYdB++12Onxee/QMwvf4/Dk/Lfp/i4nxTXtOoQ4pW5Aj7wpici1A9erdAN2OH64x8OSP9j3Ft3b7aWkTg/Fm91siTra0f9on5sQr9INejH6CUUUpavjFNq1B+Oadhxmnfa8RfEmN8VNAsQhPqF55xHkMzz3jSmChWU6f7/XZKNH+9+hBLOHYozuKQPxyMPUKkrX/K0uWnfFaJGS1QPRtZsOPtr3NsW0uyh6NNCOkU3Yz+bXbT3I8G3xE5EXLXtCXbbqwCO9zPQYPRTZ5vIDXD7U+w7rFDEoUUf7ibHIR4y6bLVPXrz8JVZEql13trxwue/uDivd3fkWRbS6/IA2bID4uk0UpF1N8qLlbBlXs4Ee7HLTfV1j54APvODnSfOWBqtKVvjgLKzF5YdEk5ewRkGlK0i33Eofffc7HT56jD7/6U+qH3Cx7SBLNntH5YIPvODnyfIXZYRVDPqgHtLs5ABHD3YzLuespb7t79FY34DjMwrVrcTuwlT55YMPvOBnRrJ4VXTdNnYug5ucHLBjEpt30701A3Ts+HEa73u6dT3FNWwflY86eMHPk+Yu+i6pzUpRrW7SNDg5JHR4KapmM5Wv2E8Tfcb1HoqqHMHU+uWDD7zg54mz5/2BSnizi9T1Dg4QQXLToGNCkb6tb1NU+QAlGr1++eADrzhn/u8Q2YZhQVlZ5+CAOtqfbhmaUCS1ezNFVm2imDbPmPng5wmz+gwh+oHDce0eUtQ6OGDIyR0uUhUsoO3vfDmmgOezH0mZN59x7MBi++WDL1g/eEiU3avlidO671bkLfwbw5XV2P8Pzo0ydy4t2/0eu33xYSOMOD8hTf4CrBtGMSoXfPLchX+J0ruSePw3LZeK0juPJbYzrhkH0io7B3k164hiGvawhOKMLkrQLyVpZg8rHFW7E2uHOL888IBPlNZ1FPzstSJM694fWr6RwpvcJK60+0HCILTBzZLFNdtAzJaohze60T8qBzyh5ZuOg5e7uwQppofEmf2++DYvmySqGBuKaicF1blQjhuHdvCIMvp8whTTfZzI7RldpwtSzL+F1+wkdZ2TBOW2gIF88PBTzD/gpeREAMEbxnJcaJHNHrpzji0gQCS6hdkEeYt9DF/2qPcEC8RM28Hwmr3sdNyht00byAut2k3gufWNtgtOEOFGUwcXWNDbdNbpgBGxEvKkOQsxivJx33iow0Vw5S6SVTrpVq11ysA2Rp7gTfPfktc6zhtXBBC+adRLshf6sG2RfHPZ5EAc4sVZ83yCN00Fk/4kggu40ZTvIEm5g24qtU4KjBrx/BTTH8ifVASAG7gKrnWxJDcU7x8X6Ecczhm3o6YicvsLXWfh3Ch1W0k8x0nXF+0fFxgt4phz8QvypiwCCFKMqXCnqXExjq10beH+UUA7+nG6mdG/Pu0f3LgFcGrl2s0kNNjpmoJ9o4B29CMO8dMT4Q5ox8uitF6fqsrJOr8qnwNbRzv6hSnG5wP+64C7h9lp30hKNtKdWjtdkbuPA19nJ7Tz3zR/ibgARbhb4AlhavcBebmTHcFl2fvYEnW0ox9xMxKBS8btJ+KiEbq9zA4RthQXDhPa0T9TEe69gWupwc6uBUphquXgf+/FrIjweHQS4/pduMe5ERUMHUd9xv8ZR98CxkS4F2n3EUrUZ10EYNw7BWm9x1GiPssi3GgiGRDKWRYZfXlON+dfNbM+GgIwYdwAAAAASUVORK5CYII=',
            shadowUrl: 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAACkAAAApCAQAAAACach9AAACMUlEQVR4Ae3ShY7jQBAE0Aoz/f9/HTMzhg1zrdKUrJbdx+Kd2nD8VNudfsL/Th///dyQN2TH6f3y/BGpC379rV+S+qqetBOxImNQXL8JCAr2V4iMQXHGNJxeCfZXhSRBcQMfvkOWUdtfzlLgAENmZDcmo2TVmt8OSM2eXxBp3DjHSMFutqS7SbmemzBiR+xpKCNUIRkdkkYxhAkyGoBvyQFEJEefwSmmvBfJuJ6aKqKWnAkvGZOaZXTUgFqYULWNSHUckZuR1HIIimUExutRxwzOLROIG4vKmCKQt364mIlhSyzAf1m9lHZHJZrlAOMMztRRiKimp/rpdJDc9Awry5xTZCte7FHtuS8wJgeYGrex28xNTd086Dik7vUMscQOa8y4DoGtCCSkAKlNwpgNtphjrC6MIHUkR6YWxxs6Sc5xqn222mmCRFzIt8lEdKx+ikCtg91qS2WpwVfBelJCiQJwvzixfI9cxZQWgiSJelKnwBElKYtDOb2MFbhmUigbReQBV0Cg4+qMXSxXSyGUn4UbF8l+7qdSGnTC0XLCmahIgUHLhLOhpVCtw4CzYXvLQWQbJNmxoCsOKAxSgBJno75avolkRw8iIAFcsdc02e9iyCd8tHwmeSSoKTowIgvscSGZUOA7PuCN5b2BX9mQM7S0wYhMNU74zgsPBj3HU7wguAfnxxjFQGBE6pwN+GjME9zHY7zGp8wVxMShYX9NXvEWD3HbwJf4giO4CFIQxXScH1/TM+04kkBiAAAAAElFTkSuQmCC',
            iconSize:     [25, 41], // size of the icon
            shadowSize:   [41, 41], // size of the shadow
            iconAnchor:   [12, 41], // point of the icon which will correspond to marker's location
            shadowAnchor: [12, 41],  // the same for the shadow
        });
        L.Marker.prototype.options.icon = customDefault;
        L.tileLayer('https://a.tile.openstreetmap.org/{z}/{x}/{y}.png', {
            maxZoom: 19,
        }).addTo(this.globalMap);
        
        this.list_photos_to_show.forEach((photo) => {
            if (photo.extra_photo_gps_lat && photo.extra_photo_gps_lon)
            {
                var takenDate = photo.extra_photo_taken_date;
                var thumbnailAddr = this.baseUThumbnail + this.userToken + '?photoId=' + photo.photo_id;
                var normalAddr = this.baseUPhoto + this.userToken + '?photoId=' + photo.photo_id;
                var insideHTML = "<h2>" + photo.photo_name + "</h2><p>" + takenDate + "</p>" + "<img src='"+thumbnailAddr+"' style='width:100px;' onerror='this.onerror=null;this.src='"+ normalAddr +"''/>";
                
                var openMap = "&nbsp<div><a href='https://maps.google.com/?q="+photo.extra_photo_gps_lat+","+photo.extra_photo_gps_lon+"' target='_blank'>Open in Map</a></div>";

                var openModal = "&nbsp<div><button onclick='openShowPhotoModal("+photo.photo_id+")'>See Photo</button></div>";

                myRepeatingMarkers.addMarker(L.marker([photo.extra_photo_gps_lat, photo.extra_photo_gps_lon], {
                    draggable: false,
                    title: photo.photo_name
                }).addTo(this.globalMap).bindPopup(insideHTML + openMap + openModal));
            }
        })
        


        setTimeout(() => {this.globalMap.invalidateSize();}, 100);
        this.showPhotoIsLoaded = true;
      
    },
    getMapPhotoList() {
      axios({
          method: 'get',
          baseURL: '',
          url: import.meta.env.VITE_APP_BASE_URL + "/photo-extra/getGlobalMapPhoto",
          headers: {
            "HRD-Token": localStorage.getItem("HRD-Token")
          },
          data: {
            
        }
        }).then((response) => {
          if (response.data.msgCode === 200)
          {
            this.list_photos_to_show = response.data.t;
            this.setUpMap();
          }else {
            window.$message.warning("No Info")
          }
        })
        .catch(function (error) { // 请求失败处理
          window.$message.error(error.$message)
          console.log(error)
        });
    },
  },
  computed: {

  },
  created() {
    window.openShowPhotoModal = this.openShowPhotoModal;
  },
  mounted () {
    this.getMapPhotoList();
  },
  components: {

  },
  props: {
    
  }
})

</script>

<style scoped>

</style>
