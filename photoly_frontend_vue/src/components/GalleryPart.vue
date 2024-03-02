<script setup>
import axios from "axios";
import {Archive16Regular as ArchiveIcon, FolderAdd20Regular, FolderArrowRight20Regular, ArrowUpload20Filled,
  ArrowDownload20Filled, Eye20Regular, Delete20Regular, Edit20Regular } from "@vicons/fluent"
import {Star20Regular as Star, Airplane20Regular as Airplane, Flag20Regular as Flag, ImageMultiple20Regular as Album, Heart20Regular as Heart,
  Luggage20Regular as Luggage,  AnimalDog20Regular as Animal, Archive20Regular as Archive, Beach20Regular as Beach, Globe20Regular as Global, Album20Regular as AlbumDefault } from "@vicons/fluent"
  
import { ref, defineComponent, nextTick, h } from 'vue';
import { NTag } from "naive-ui";
import { useMessage, NIcon } from 'naive-ui'
import MoveToShowFolder from "./MoveToShowFolder.vue"
import MediaList from '@/components/MediaList.vue'
</script>

<template>
  <div>

    <!-- This part is for medias -->

    <div @contextmenu="blankSpaceMenu" >

      <!-- This part is for loading photos cards into the "media" section -->

      <div style="margin-top: 1rem;">
        <div style="display: flex; align-items: flex-start; flex-direction: row;">

          <div style="width: 15vw;">
            <n-h2>Galleries</n-h2>
            <n-collapse :default-expanded-names="['show']">

              <n-collapse-item title="Add New Gallery" name="add">
                <n-card title="" size="small" >
                      Add a New Gallery
                      <template #footer>
                        <n-space vertical>
                          <n-input v-model:value="addNewGalleryInput" type="text" placeholder="Gallery Name" @keyup.enter="createGa" />
                          <n-select v-model:value="addNewGalleryIconSelectVal" :options="addNewGalleryIconSelectOptions" />
                          <n-color-picker v-model:value="addNewGalleryIconColorVal" :show-alpha="false" :modes="['hex']" />
                          <n-button strong secondary style="width: 100%; " @click="createGa">
                            Add
                          </n-button>
                        </n-space>
                      </template>
                </n-card>
              </n-collapse-item>

              <n-collapse-item title="Galleries" name="show">
                <n-flex vertical>
                  <n-card class="galleryCard" title="" size="small" v-for="gallery in galleryChildren" :style="{'background-color' : renderPhotoGaId === gallery.gaId ? '#8fd2ce' : '#EDF2F7', 'border-radius': '7px'}"
                    @click.left.native="selectGallery(gallery)" @click.right.native="GalleryMenu(gallery, $event)">
                    <n-flex>
                      <n-p>{{ gallery.gaName.length > 15 ? gallery.gaName.substring(0, 15) + "..." : gallery.gaName }}</n-p>
                      <n-icon size="25" :color="gallery.coverColor" :component="renderGaIcon(gallery.coverId)"></n-icon>
                    </n-flex>
                    Created: {{ gallery.createDate.split('T')[0] }}
                  </n-card>
                </n-flex>
              </n-collapse-item>

            </n-collapse>
            
          </div>

          <MediaList style="margin-left: 2rem;" @queryPhotos="queryPhotos" @doDetach="removeFromGallerySelectedPhoto" ref="mediaListRef" :nsOrNot="false" :detachEnabled="true" detachStr="Gallery"/>

        </div>
        
      </div>
      <!-- ENDOF: This part is for loading photos cards into the "media" section -->

    </div>
    <!-- ENDOF: This part is for medias -->

    <!-- MENU PART -->
    <!-- This part is for redesign of right click menu for gallery part -->
    <n-dropdown
        placement="bottom-start"
        trigger="manual"
        :x="mouseX"
        :y="mouseY"
        :options="GalleryMenuOptions"
        :show="showGalleryMenu"
        :on-clickoutside="onClickOutsideGallery"
        @select="handleSelectGallery"
      />
    <!-- ENDOF: This part is for redesign of right click menu for gallery part -->
    <!-- ENDOF: MENU PART -->

    <!-- MODAL PART -->
    <!-- This part is for modal of confirming to delete a gallery -->
    <n-modal v-model:show="showDeleteGalleryConfirmModal">
      <n-card
        style="width: 400px; border-radius: 10px;"
        title="Confirm to Delete"
        :bordered="false"
        size="huge"
        role="dialog"
        aria-modal="true"
      >
        Do you really want to DELETE this gallery?
        <p>*All photos associated with this gallery will be detached (photos will not been deleted).</p>

        <n-space style="margin-top: 1rem">
          <n-button type="error" id="bt1" round size="large" style="margin-top: 1rem; width: 100%;" @click="deleteGallery">
            Delete
          </n-button>
          <n-button id="bt1" round size="large" style="margin-top: 1rem; width: 100%;" @click="() => {showDeleteGalleryConfirmModal = false;}">
            Cancel
          </n-button>
        </n-space>

      </n-card>
    </n-modal>
    <!-- ENDOF: This part is for modal of confirming to delete a gallery -->

    <!-- This part is for modal of edit a gallery's information -->
    <n-modal v-model:show="showEditGalleryModal">
      <n-card title="Edit Gallery" size="huge" style="width: 400px; border-radius: 10px;" role="dialog"
        aria-modal="true" :bordered="false">
            <n-space vertical>
                <n-input v-model:value="editGalleryInfo.gaName" type="text" placeholder="Gallery Name" @keyup.enter="editGa"/>
                <n-select v-model:value="editGalleryInfo.coverId" :options="addNewGalleryIconSelectOptions" />
                <n-color-picker v-model:value="editGalleryInfo.coverColor" :show-alpha="false" :modes="['hex']"/>
                <n-button strong secondary style="width: 100%; " @click="editGa">
                  Submit
                </n-button>
            </n-space>
      </n-card>
    </n-modal>
    <!-- ENDOF: This part is for modal of edit a gallery's information -->
    
    
    <!-- ENDOF: MODAL PART -->

    <n-back-top :right="100" />
  </div>

  
  
</template>

<script>
const mediaListRef = ref(null);
export default defineComponent({
  data() {
    return {
      renderPhotoGaId: -1,
      nsPathStr: "",
      userToken: localStorage.getItem("HRD-Token"),
      showGalleryMenu: false,
      showPhotoSpaceMenuPhotoInfo: null,
      mouseX: null,
      mouseY: null,
      GalleryMenuOptions: [
        {
          label: "Edit",
          key: "edit",
          icon() {
            return h(NIcon, null, {
              default: () => h(Edit20Regular)
            });
          }
        },
        {
          label: "Delete",
          key: "delete",
          icon() {
            return h(NIcon, null, {
              default: () => h(Delete20Regular)
            });
          }
        },
        {
          label: "Download",
          key: "download",
          icon() {
            return h(NIcon, null, {
              default: () => h(ArrowDownload20Filled)
            });
          }
        },
      ],
      
      galleryChildren: [],
      addNewGalleryInput: "",
      addNewGalleryIconSelectVal: 0,
      addNewGalleryIconSelectOptions: [
        {
            label: "Default",
            value: 0
        },
        {
            label: "Heart",
            value: 1
        },
        {
            label: "Star",
            value: 2
        },
        {
            label: "Airplane",
            value: 3
        },
        {
            label: "Flag",
            value: 4
        },
        {
            label: "Album",
            value: 5
        },
        {
            label: "Luggage",
            value: 6
        },
        {
            label: "Animal",
            value: '7'
        },
        {
            label: "Archive",
            value: '8'
        },
        {
            label: "Beach",
            value: '9'
        },
        {
            label: "Globe",
            value: '10'
        },
      ],
      addNewGalleryIconColorVal: "#000000",

      editGalleryInfo: null,

      showDeleteGalleryConfirmModal: false,
      showEditGalleryModal: false,
    }
  },
  setup() {
    
    return {
      
    };
  },
  methods: {
    downloadGa () {
      axios
          .get(import.meta.env.VITE_APP_BASE_URL + "/gallery/downloadGa/" + this.userToken + "?gaId="+ this.editGalleryInfo.gaId, {responseType: "blob"})
          .then(async (response) => {
            var a = document.createElement("a");
            document.body.appendChild(a);
            a.style = "display: none";

            let url = window.URL.createObjectURL(new Blob([response.data]));

            let link = document.createElement("a");
            link.style.display = "none";
            link.href = url;
            link.download = this.editGalleryInfo.gaName + ".zip";
            document.body.appendChild(link);
            link.click();
          })
          .catch(function (error) { // 请求失败处理
            // console.log(error);
            window.$message.error(error);
          });
    },
    editGa() {
      if (this.editGalleryInfo.gaName === null || this.editGalleryInfo.gaName === "")
      {
        window.$message.warning("Please Input a Valid Gallery Name!");
      }else {
        axios({
          method: 'post',
          baseURL: '',
          url: import.meta.env.VITE_APP_BASE_URL + "/gallery/update",
          headers: {
            "HRD-Token": localStorage.getItem("HRD-Token")
          },
          data: this.editGalleryInfo
        }).then((response) => {
          if (response.data.msgCode === 200)
          {
            window.$message.success("Information Updated!");
            this.getAllGas();
            this.showEditGalleryModal = false;
          }else {
            window.$message.error("Update Response Error!")
          }
        })
        .catch(function (error) { // 请求失败处理
          // console.log(error);
          window.$message.error(error);
        });
      }
      
    },
    createGa() {
      if (this.addNewGalleryInput === null || this.addNewGalleryInput === "")
      {
        window.$message.warning("Please Input a Valid Gallery Name!");
      }else {
        axios({
          method: 'post',
          baseURL: '',
          url: import.meta.env.VITE_APP_BASE_URL + "/gallery/insert",
          headers: {
            "HRD-Token": localStorage.getItem("HRD-Token")
          },
          data: {
            "gaName": this.addNewGalleryInput,
            "coverId": this.addNewGalleryIconSelectVal,
            "coverColor": this.addNewGalleryIconColorVal
        }
        }).then((response) => {
          if (response.data.msgCode === 200)
          {
            window.$message.success("Created a New Gallery!");
            this.getAllGas();
            this.addNewGalleryInput = "";
            this.addNewGalleryIconSelectVal = 0;
            this.addNewGalleryIconColorVal = "#000000";
          }else {
            window.$message.error("createGa Response Error!")
          }
        })
        .catch(function (error) { // 请求失败处理
          // console.log(error);
          window.$message.error(error);
        });
      }
      
    },
    selectGallery(gallery) {
      if (this.renderPhotoGaId == gallery.gaId)
      {
        this.renderPhotoGaId = -1;
        mediaListRef.value.updatePhotoList([]);
      }else {
        this.renderPhotoGaId = gallery.gaId;
        this.queryPhotos();
      }
    },
    removeFromGallerySelectedPhoto (photoChildren) {
      photoChildren.forEach((photo) => {
        if(photo["checked"])
        {
          axios({
            method: 'post',
            baseURL: '',
            url: import.meta.env.VITE_APP_BASE_URL + "/photo/deleteFromGallery?photoId=" + photo.photoId + "&gaId=" + this.renderPhotoGaId,
            headers: {
              "HRD-Token": localStorage.getItem("HRD-Token")
            },
            data: {
          }
          }).then((response) => {
            if (response.data.msgCode === 200)
            {
              this.queryPhotos();
              window.$message.success("Detached from gallery!");
              
            }else {
              window.$message.error("removeGa Response Error!")
            }
          })
          .catch(function (error) { // 请求失败处理
            // console.log(error);
            window.$message.error(error);
          });
            }
          })
      this.showPhotoMultiDetachModal = false;
      this.photoMultiSelectMode = false;
    },
    blankSpaceMenu (e) {
      e.preventDefault();
      e.stopPropagation();
    },
    handleSelectGallery(key) {
      this.showGalleryMenu = false;
      if (key === "edit")
      {
        this.showEditGalleryModal = true;
      }
      if (key === "delete")
      {
        this.showDeleteGalleryConfirmModal = true;
      }
      if (key === "download")
      {
        this.downloadGa();
      }
    },
    onClickOutsideGallery () {
      this.showGalleryMenu = false;
    },
    GalleryMenu(gallery, e) {
      this.editGalleryInfo = {
        "gaId": gallery.gaId,
        "gaName": gallery.gaName,
        "userId": gallery.userId,
        "createDate": gallery.gallery,
        "coverId": gallery.coverId,
        "coverColor": gallery.coverColor
      };
      e.preventDefault();
      e.stopPropagation();
      this.mouseX = e.clientX;
      this.mouseY = e.clientY;
      this.showGalleryMenu = true;
    },
    renderGaIcon (coverId) {
      if (coverId === null)
      {
        return AlbumDefault
      }
      switch (coverId)
      {
        case 0:
          return AlbumDefault
        case 1:
            return Heart
        case 2:
          return Star
        case 3:
          return Airplane
        case 4:
          return Flag
        case 5:
          return Album
        case 6:
          return Luggage
        case 7:
          return Animal
        case 8:
          return Beach
        case 9:
          return Global
      }
    },
    getAllGas() {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/gallery/getAll",
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        
        if (response.data.msgCode === 200)
        {
          var infoT = response.data.t;
          let allgas = [];
          for(var i = 0; i < infoT.length; i ++)
          {
            var gas = infoT[i];
            allgas.push({
              "label": gas["gaName"], 
              "value": gas["gaId"]
            })
          }
          this.gaOptions = allgas;
          this.galleryChildren = infoT;
        }else {
          window.$message.error("getGa Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        window.$message.error(error);
      });
    },
    removeGa(gaId) {
      axios({
        method: 'post',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/photo/deleteFromGallery?photoId=" + this.showPhotoInfo.photoId + "&gaId=" + gaId,
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
      }
      }).then((response) => {
        if (response.data.msgCode === 200)
        {
          this.getGas();
          this.queryPhotos();
          window.$message.success("Gallery detached!")
        }else {
          window.$message.error("removeGa Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        // console.log(error);
        window.$message.error(error);
      });
    },
    deleteGallery () {
      axios({
          method: 'post',
          baseURL: '',
          url: import.meta.env.VITE_APP_BASE_URL + "/gallery/delete?gaId=" + this.editGalleryInfo.gaId,
          headers: {
            "HRD-Token": localStorage.getItem("HRD-Token")
          },
          data: {
        }
        }).then((response) => {
          if (response.data.msgCode === 200)
          {
            this.getAllGas();
            if (this.renderPhotoGaId === this.editGalleryInfo.gaId)
            {
              this.photoChildren = []
            }
            this.showDeleteGalleryConfirmModal = false;
            window.$message.success("Gallery Deleted!");
          }else {
            window.$message.warning("Update Error!")
          }
        })
        .catch(function (error) { // 请求失败处理
          window.$message.error(error);
        });
    },
    queryPhotos() {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/photo/getByGallery?gaId=" + this.renderPhotoGaId,
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
        }else {
          window.$message.error("query photo Response Error!")
        }
      })
      .catch(function (error) { // 请求失败处理
        // console.log(error);
        window.$message.error(error);
      });
    },
  },
  computed: {

  },
  mounted () {
    window.$message = useMessage();
    this.getAllGas();
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
.galleryCard {
  cursor: pointer;
  user-select: none;
}
</style>
