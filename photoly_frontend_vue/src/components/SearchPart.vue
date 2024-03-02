<script setup>
import axios from "axios";
import {Archive16Regular as ArchiveIcon, FolderAdd20Regular, FolderArrowRight20Regular, ArrowUpload20Filled,
  ArrowDownload20Filled, Eye20Regular, Delete20Regular, Edit20Regular } from "@vicons/fluent"
import { ref, defineComponent, nextTick, h } from 'vue';
import { NTag } from "naive-ui";
import { useMessage, NIcon } from 'naive-ui'
import MoveToShowFolder from "./MoveToShowFolder.vue"
import MediaList from '@/components/MediaList.vue'
</script>

<template>
  <div>

    <!-- This part is for medias -->

    <div @contextmenu="blankSpaceMenu">

      <!-- This part is for loading photos cards into the "media" section -->

      <div style="margin-top: 1rem;">
        <div style="display: flex; align-items: flex-start; flex-direction: column;">

          <div>
            <n-h2>Search</n-h2>
            <n-card title="Query Information" style="width: 60vw; border-radius: 10px;" size="huge">
              <n-input-group style="width: 50vw; ">
                <n-select :style="{ width: '30%' }" :options="searchOptions" v-model:value="searchTypeSelectVal" />
                <n-input :style="{ width: '60%' }" v-model:value="searchInput" clearable @keyup.enter="queryPhotos" />
                <n-button :style="{ width: '20%' }" type="success" ghost @click="queryPhotos">
                  Search
                </n-button>
              </n-input-group>
              <template #footer>
                The search utilizes partial or full match.
              </template>
            </n-card>
            
            
          </div>

          <MediaList style="margin-left: 2rem;" @queryPhotos="queryPhotos" ref="mediaListRef" :nsOrNot="false" :detachEnabled="false"/>

        </div>
        
      </div>
      <!-- ENDOF: This part is for loading photos cards into the "media" section -->

    </div>
    <!-- ENDOF: This part is for medias -->

    <n-back-top :right="100" />
  </div>

  
  
</template>

<script>

const mediaListRef = ref(null);

export default defineComponent({
  data() {
    return {
      nsPathStr: "",
      userToken: localStorage.getItem("HRD-Token"),
      
      searchTypeSelectVal: "photoName",
      searchInput: "",
      searchOptions: [
        {
          label: 'Photo Name',
          value: 'photoName'
        },
        {
          label: 'Tag',
          value: 'tag'
        },
        {
          label: 'Gallery',
          value: 'gallery'
        },
        {
          label: 'Namespace',
          value: 'namespace'
        },
      ],
    }
  },
  setup() {
    
    return {
      
    };
  },
  methods: {
    blankSpaceMenu (e) {
      e.preventDefault();
      e.stopPropagation();
    },
    queryPhotos() {
      if (this.searchInput === null || this.searchInput === "" || this.searchInput === " ")
      {
        mediaListRef.value.updatePhotoList([]);
      }else {
        axios({
          method: 'get',
          baseURL: '',
          url: import.meta.env.VITE_APP_BASE_URL + "/photo/searchPhoto?option=" + this.searchTypeSelectVal + "&query=" + this.searchInput,
          headers: {
            "HRD-Token": localStorage.getItem("HRD-Token")
          },
          data: {
        }
        }).then((response) => {
          if (response.data.msgCode === 200)
          {
            var infoT = response.data.t;
            mediaListRef.value.updatePhotoList(infoT);
            window.$message.success(infoT.length + " record(s) found")
          }else if (response.data.msgCode === 404) {
            window.$message.info("No record found")
          }else {
            window.$message.error("query photo Response Error!")
          }
        })
        .catch(function (error) { // 请求失败处理
          // console.log(error);
          window.$message.error(error);
        });
      }
      
    },
  },
  computed: {

  },  
  mounted () {
    window.$message = useMessage();
  },
  components: {

  }
})

</script>

<style scoped>
.n-button {
  border-radius: 10px;
  width: 15rem;
  
}
.cardImgDiv {
  width: 255px;
  height: 128px;
}
.cardImg {
  object-fit: cover;
  width: 100%;
  max-width: 100%;
  height: 128px;
  overflow: hidden;
}
.n-card {
  /*cursor: pointer;*/
  padding: 0;
}
</style>
